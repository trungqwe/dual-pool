package configtxn

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/trungqwe/dual-pool/internal/lockfile"
	"github.com/trungqwe/dual-pool/internal/state"
)

type FaultPoint string

const (
	AfterBackupSync         FaultPoint = "AFTER_BACKUP_SYNC"
	AfterCandidateSync      FaultPoint = "AFTER_CANDIDATE_SYNC"
	AfterMarkerSync         FaultPoint = "AFTER_MARKER_SYNC"
	AfterPendingOwnership   FaultPoint = "AFTER_PENDING_OWNERSHIP"
	BeforeTargetCAS         FaultPoint = "BEFORE_TARGET_CAS"
	AfterTargetReplace      FaultPoint = "AFTER_TARGET_REPLACE"
	AfterTargetVerify       FaultPoint = "AFTER_TARGET_VERIFY"
	AfterTargetSync         FaultPoint = "AFTER_TARGET_SYNC"
	BeforeOwnershipFinalize FaultPoint = "BEFORE_OWNERSHIP_FINALIZE"
	AfterOwnershipFinalize  FaultPoint = "AFTER_OWNERSHIP_FINALIZE"
	BeforeMarkerCleanup     FaultPoint = "BEFORE_MARKER_CLEANUP"
)

type Option func(*Engine)

func WithFaultInjector(f func(FaultPoint) error) Option { return func(e *Engine) { e.fault = f } }
func WithTransactionIDGenerator(f func() (string, error)) Option {
	return func(e *Engine) { e.transactionID = f }
}
func WithReplacementAPI(r replacementAPI) Option { return func(e *Engine) { e.replacer = r } }
func WithRemoveFile(f func(string) error) Option { return func(e *Engine) { e.removeFile = f } }

type Engine struct {
	journalDir, backupDir string
	locks                 *lockfile.Manager
	store                 *state.Store
	replacer              replacementAPI
	fault                 func(FaultPoint) error
	transactionID         func() (string, error)
	now                   func() time.Time
	toolVersion           string
	removeFile            func(string) error
}

func NewEngine(journalDir, backupDir string, locks *lockfile.Manager, store *state.Store, options ...Option) (*Engine, error) {
	if locks == nil || store == nil {
		return nil, ErrPersistence
	}
	j, err := safeExistingDir(journalDir)
	if err != nil {
		return nil, err
	}
	b, err := safeExistingDir(backupDir)
	if err != nil {
		return nil, err
	}
	e := &Engine{journalDir: j, backupDir: b, locks: locks, store: store, replacer: windowsReplacement{}, transactionID: randomID, now: time.Now, toolVersion: "dev", removeFile: os.Remove}
	for _, o := range options {
		o(e)
	}
	if e.replacer == nil || e.transactionID == nil || e.removeFile == nil {
		return nil, ErrPersistence
	}
	return e, nil
}

func (e *Engine) Apply(plan CodexPlan) (result error) {
	global, err := e.locks.AcquireGlobal()
	if err != nil {
		return err
	}
	defer func() {
		if x := global.Release(); result == nil && x != nil {
			result = x
		}
	}()
	target, err := safeTarget(plan.Target)
	if err != nil {
		return err
	}
	resource := resourceID(target)
	markerPath := e.markerPath(resource)
	if existsSafe(markerPath) {
		return ErrRecoveryRequired
	}
	ownership, err := e.loadOwnership()
	if err != nil {
		return err
	}
	existing := recordsFor(ownership, target)
	if len(existing) > 0 {
		return e.idempotentApply(target, plan.desired(), existing)
	}
	originalBytes, err := os.ReadFile(target)
	if err != nil {
		return ErrPersistence
	}
	parsed, err := parseDocument(originalBytes)
	if err != nil {
		return err
	}
	candidateBytes, original, err := parsed.patch(plan.desired())
	if err != nil {
		return err
	}
	verified, err := parseDocument(candidateBytes)
	if err != nil || !semanticMatches(verified.semantic, plan.desired()) {
		return ErrUnsupportedConfigShape
	}
	id, err := e.transactionID()
	if err != nil || !hex32.MatchString(id) {
		return ErrPersistence
	}
	preHash := hash(originalBytes)
	postHash := hash(candidateBytes)
	identity, err := fileIdentity(target)
	if err != nil {
		return err
	}
	backupBase := "config-" + id + ".bak"
	backup := filepath.Join(e.backupDir, backupBase)
	candidate := filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".candidate-"+id)
	if err = writeVerifiedExclusive(backup, originalBytes); err != nil {
		return err
	}
	if err = e.inject(AfterBackupSync); err != nil {
		return err
	}
	if err = writeVerifiedExclusive(candidate, candidateBytes); err != nil {
		return err
	}
	if err = e.inject(AfterCandidateSync); err != nil {
		return err
	}
	keys := sortedKeys(plan.desired())
	m := marker{1, "apply", id, resource, preHash, postHash, backupBase, preHash, keys}
	markerBytes, _ := encodeMarker(m)
	if err = writeVerifiedExclusive(markerPath, markerBytes); err != nil {
		return err
	}
	if err = e.inject(AfterMarkerSync); err != nil {
		return err
	}
	records := makeRecords(target, identity, parsed.meta, preHash, postHash, backup, preHash, original, plan.desired(), e.now().UTC().Format(time.RFC3339), e.toolVersion, state.RollbackPending)
	ownership.Records = append(ownership.Records, records...)
	if err = e.store.SaveOwnership(ownership); err != nil {
		return ErrPersistence
	}
	if err = e.inject(AfterPendingOwnership); err != nil {
		return err
	}
	guard, err := e.locks.AcquireFile(target)
	if err != nil {
		return err
	}
	defer guard.Release()
	if err = e.inject(BeforeTargetCAS); err != nil {
		return err
	}
	if !fileHashEquals(target, preHash) {
		return ErrConfigConflict
	}
	if err = e.replacer.replace(target, candidate); err != nil {
		return err
	}
	if err = e.inject(AfterTargetReplace); err != nil {
		return err
	}
	final, err := os.ReadFile(target)
	if err != nil || hash(final) != postHash {
		return ErrRecoveryUnresolved
	}
	fp, err := parseDocument(final)
	if err != nil || !semanticMatches(fp.semantic, plan.desired()) {
		return ErrRecoveryUnresolved
	}
	if err = e.inject(AfterTargetVerify); err != nil {
		return err
	}
	if err = syncPath(target); err != nil {
		return err
	}
	if err = e.inject(AfterTargetSync); err != nil {
		return err
	}
	if err = guard.Release(); err != nil {
		return err
	}
	if err = e.inject(BeforeOwnershipFinalize); err != nil {
		return err
	}
	setStatus(&ownership, target, state.RollbackPending, state.RollbackApplied)
	if err = e.store.SaveOwnership(ownership); err != nil {
		return ErrPersistence
	}
	if err = e.inject(AfterOwnershipFinalize); err != nil {
		return err
	}
	if err = e.inject(BeforeMarkerCleanup); err != nil {
		return err
	}
	if e.removeRequired(markerPath) != nil {
		return ErrPersistence
	}
	return nil
}

func (e *Engine) Rollback(plan CodexPlan) (result error) {
	global, err := e.locks.AcquireGlobal()
	if err != nil {
		return err
	}
	defer func() {
		if x := global.Release(); result == nil && x != nil {
			result = x
		}
	}()
	target, err := safeTarget(plan.Target)
	if err != nil {
		return err
	}
	resource := resourceID(target)
	markerPath := e.markerPath(resource)
	if existsSafe(markerPath) {
		return ErrRecoveryRequired
	}
	ownership, err := e.loadOwnership()
	if err != nil {
		return err
	}
	records := recordsFor(ownership, target)
	if len(records) == 0 {
		return nil
	}
	allRolled := true
	allConflict := true
	for _, r := range records {
		if r.RollbackStatus != state.RollbackRolledBack {
			allRolled = false
		}
		if r.RollbackStatus != state.RollbackConflict {
			allConflict = false
		}
	}
	if allRolled {
		return nil
	}
	if allConflict {
		return ErrRollbackConflict
	}
	for _, r := range records {
		if r.RollbackStatus != state.RollbackApplied {
			return ErrRecoveryRequired
		}
	}
	current, err := os.ReadFile(target)
	if err != nil {
		return ErrPersistence
	}
	parsed, err := parseDocument(current)
	if err != nil {
		return err
	}
	applied := valuesFromRecords(records, false)
	if !semanticMatches(parsed.semantic, applied) {
		guard, lockErr := e.locks.AcquireFile(target)
		if lockErr != nil {
			return lockErr
		}
		lockedBytes, readErr := os.ReadFile(target)
		lockedParsed, parseErr := parseDocument(lockedBytes)
		if readErr != nil || parseErr != nil {
			_ = guard.Release()
			return ErrPersistence
		}
		if !semanticMatches(lockedParsed.semantic, applied) {
			setStatus(&ownership, target, state.RollbackApplied, state.RollbackConflict)
			if err = guard.Release(); err != nil {
				return err
			}
			if err = e.store.SaveOwnership(ownership); err != nil {
				return ErrPersistence
			}
			return ErrRollbackConflict
		}
		if err = guard.Release(); err != nil {
			return err
		}
		current, parsed = lockedBytes, lockedParsed
	}
	original := valuesFromRecords(records, true)
	candidateBytes, err := parsed.restore(original)
	if err != nil {
		return err
	}
	verified, err := parseDocument(candidateBytes)
	if err != nil || !semanticMatches(verified.semantic, original) {
		return ErrUnsupportedConfigShape
	}
	id, err := e.transactionID()
	if err != nil || !hex32.MatchString(id) {
		return ErrPersistence
	}
	preHash := hash(current)
	postHash := hash(candidateBytes)
	backupBase := "config-" + id + ".bak"
	backup := filepath.Join(e.backupDir, backupBase)
	candidate := filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".candidate-"+id)
	if err = writeVerifiedExclusive(backup, current); err != nil {
		return err
	}
	if err = e.inject(AfterBackupSync); err != nil {
		return err
	}
	if err = writeVerifiedExclusive(candidate, candidateBytes); err != nil {
		return err
	}
	if err = e.inject(AfterCandidateSync); err != nil {
		return err
	}
	m := marker{1, "rollback", id, resource, preHash, postHash, backupBase, preHash, recordKeys(records)}
	b, _ := encodeMarker(m)
	if err = writeVerifiedExclusive(markerPath, b); err != nil {
		return err
	}
	if err = e.inject(AfterMarkerSync); err != nil {
		return err
	}
	guard, err := e.locks.AcquireFile(target)
	if err != nil {
		return err
	}
	defer guard.Release()
	if err = e.inject(BeforeTargetCAS); err != nil {
		return err
	}
	if !fileHashEquals(target, preHash) {
		return ErrConfigConflict
	}
	if err = e.replacer.replace(target, candidate); err != nil {
		return err
	}
	if err = e.inject(AfterTargetReplace); err != nil {
		return err
	}
	final, err := os.ReadFile(target)
	if err != nil || hash(final) != postHash {
		return ErrRecoveryUnresolved
	}
	if err = e.inject(AfterTargetVerify); err != nil {
		return err
	}
	if err = syncPath(target); err != nil {
		return err
	}
	if err = e.inject(AfterTargetSync); err != nil {
		return err
	}
	if err = guard.Release(); err != nil {
		return err
	}
	if err = e.inject(BeforeOwnershipFinalize); err != nil {
		return err
	}
	setStatus(&ownership, target, state.RollbackApplied, state.RollbackRolledBack)
	if err = e.store.SaveOwnership(ownership); err != nil {
		return ErrPersistence
	}
	if err = e.inject(AfterOwnershipFinalize); err != nil {
		return err
	}
	if err = e.inject(BeforeMarkerCleanup); err != nil {
		return err
	}
	if e.removeRequired(markerPath) != nil {
		return ErrPersistence
	}
	return nil
}

func (e *Engine) Recover(plan CodexPlan) (result error) {
	global, err := e.locks.AcquireGlobal()
	if err != nil {
		return err
	}
	defer func() {
		if x := global.Release(); result == nil && x != nil {
			result = x
		}
	}()
	target, err := safeTarget(plan.Target)
	if err != nil {
		return err
	}
	resource := resourceID(target)
	mp := e.markerPath(resource)
	data, err := readOwnedArtifact(mp, e.journalDir, filepath.Base(mp), true)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return ErrRecoveryUnresolved
	}
	m, err := decodeMarker(data)
	if err != nil || m.TargetResourceID != resource {
		return ErrRecoveryUnresolved
	}
	ownership, err := e.loadOwnership()
	if err != nil {
		return err
	}
	records := recordsFor(ownership, target)
	if len(records) == 0 && !(m.Operation == "apply" && currentHashOrEmpty(target) == m.PreHash) {
		return ErrRecoveryUnresolved
	}
	if len(records) > 0 && !markerMatchesRecords(m, records) {
		return ErrRecoveryUnresolved
	}
	guard, err := e.locks.AcquireFile(target)
	if err != nil {
		return err
	}
	defer guard.Release()
	current, readErr := os.ReadFile(target)
	currentHash := ""
	if readErr == nil {
		currentHash = hash(current)
	}
	backup := filepath.Join(e.backupDir, m.BackupBasename)
	backupBytes, backupErr := readOwnedArtifact(backup, e.backupDir, m.BackupBasename, true)
	if backupErr != nil || hash(backupBytes) != m.BackupHash {
		return ErrRecoveryUnresolved
	}
	if err = e.validateCleanupSet(target, m, true); err != nil {
		return err
	}
	if currentHash == m.PreHash {
		if m.Operation == "apply" {
			for _, r := range records {
				if r.RollbackStatus != state.RollbackPending {
					return ErrRecoveryUnresolved
				}
			}
			ownership.Records = removeRecords(ownership.Records, target)
			if err = guard.Release(); err != nil {
				return err
			}
			if e.store.SaveOwnership(ownership) != nil {
				return ErrPersistence
			}
		}
		if m.Operation == "rollback" {
			if err = guard.Release(); err != nil {
				return err
			}
		}
		return e.cleanupTransient(target, m, true)
	}
	if currentHash == m.PostHash {
		parsed, er := parseDocument(current)
		if er != nil {
			return ErrRecoveryUnresolved
		}
		if m.Operation == "apply" {
			for _, r := range records {
				if r.RollbackStatus != state.RollbackPending && r.RollbackStatus != state.RollbackApplied {
					return ErrRecoveryUnresolved
				}
			}
			if !semanticMatches(parsed.semantic, valuesFromRecords(records, false)) {
				return ErrRecoveryUnresolved
			}
			setStatus(&ownership, target, state.RollbackPending, state.RollbackApplied)
		} else {
			for _, r := range records {
				if r.RollbackStatus != state.RollbackApplied && r.RollbackStatus != state.RollbackRolledBack {
					return ErrRecoveryUnresolved
				}
			}
			if !semanticMatches(parsed.semantic, valuesFromRecords(records, true)) {
				return ErrRecoveryUnresolved
			}
			setStatus(&ownership, target, state.RollbackApplied, state.RollbackRolledBack)
		}
		if err = guard.Release(); err != nil {
			return err
		}
		if e.store.SaveOwnership(ownership) != nil {
			return ErrPersistence
		}
		return e.cleanupTransient(target, m, m.Operation == "rollback")
	}
	if readErr != nil || parseFailed(current) {
		restoreCandidate := filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".restore-"+m.TransactionID)
		if writeVerifiedExclusive(restoreCandidate, backupBytes) != nil {
			return ErrRecoveryUnresolved
		}
		if readErr == nil {
			err = e.replacer.replace(target, restoreCandidate)
		} else {
			err = e.replacer.install(restoreCandidate, target)
		}
		if err != nil {
			return err
		}
		if m.Operation == "apply" {
			ownership.Records = removeRecords(ownership.Records, target)
		}
		if err = guard.Release(); err != nil {
			return err
		}
		if e.store.SaveOwnership(ownership) != nil {
			return ErrPersistence
		}
		return e.cleanupTransient(target, m, m.Operation == "apply")
	}
	return ErrConfigConflict
}

func currentHashOrEmpty(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return hash(b)
}

func (e *Engine) cleanupTransient(target string, m marker, removeBackup bool) error {
	if err := e.validateCleanupSet(target, m, removeBackup); err != nil {
		return err
	}
	if err := e.removeOptional(filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".candidate-"+m.TransactionID)); err != nil {
		return ErrRecoveryRequired
	}
	if removeBackup {
		if err := e.removeOptional(filepath.Join(e.backupDir, m.BackupBasename)); err != nil {
			return ErrRecoveryRequired
		}
	}
	if err := e.removeRequired(e.markerPath(m.TargetResourceID)); err != nil {
		return ErrRecoveryRequired
	}
	return nil
}

func (e *Engine) validateCleanupSet(target string, m marker, requireBackup bool) error {
	candidate := filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".candidate-"+m.TransactionID)
	if _, err := readOwnedArtifact(candidate, filepath.Dir(target), filepath.Base(candidate), false); err != nil && !os.IsNotExist(err) {
		return ErrRecoveryUnresolved
	}
	if requireBackup {
		backup := filepath.Join(e.backupDir, m.BackupBasename)
		if _, err := readOwnedArtifact(backup, e.backupDir, m.BackupBasename, true); err != nil {
			return ErrRecoveryUnresolved
		}
	}
	markerPath := e.markerPath(m.TargetResourceID)
	if _, err := readOwnedArtifact(markerPath, e.journalDir, filepath.Base(markerPath), true); err != nil {
		return ErrRecoveryUnresolved
	}
	return nil
}

func (e *Engine) removeOptional(path string) error {
	err := e.removeFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
func (e *Engine) removeRequired(path string) error { return e.removeFile(path) }

func readOwnedArtifact(path, directory, basename string, required bool) ([]byte, error) {
	if filepath.Base(path) != basename || !strings.EqualFold(filepath.Dir(path), directory) || !safeLocalAbsolute(path) {
		return nil, ErrUnsafeConfigArtifact
	}
	b, err := readSafeRegular(path)
	if os.IsNotExist(err) && !required {
		return nil, err
	}
	return b, err
}

func (e *Engine) idempotentApply(target string, desired map[string]value, records []state.OwnershipRecord) error {
	for _, r := range records {
		if r.RollbackStatus == state.RollbackRolledBack {
			return ErrReconfigureUnsupported
		}
		if r.RollbackStatus != state.RollbackApplied {
			return ErrRecoveryRequired
		}
	}
	if !semanticMatches(valuesFromRecords(records, false), desired) {
		return ErrReconfigureUnsupported
	}
	b, err := os.ReadFile(target)
	if err != nil {
		return ErrPersistence
	}
	p, err := parseDocument(b)
	if err != nil {
		return err
	}
	if !semanticMatches(p.semantic, desired) {
		return ErrConfigConflict
	}
	return nil
}
func (e *Engine) markerPath(resource string) string {
	return filepath.Join(e.journalDir, "config-"+resource+".recovery")
}
func (e *Engine) inject(p FaultPoint) error {
	if e.fault == nil {
		return nil
	}
	return e.fault(p)
}
func (e *Engine) loadOwnership() (state.Ownership, error) {
	o, err := e.store.LoadOwnership()
	if errors.Is(err, state.ErrNotInitialized) {
		return state.Ownership{SchemaVersion: state.OwnershipSchemaVersion}, nil
	}
	if err != nil {
		return state.Ownership{}, err
	}
	return o, nil
}

func safeExistingDir(path string) (string, error) {
	if !safeLocalAbsolute(path) {
		return "", ErrUnsafeConfigArtifact
	}
	a, e := filepath.Abs(path)
	if e != nil || filepath.Clean(a) != a {
		return "", ErrUnsafeConfigArtifact
	}
	if validatePathHierarchy(a, true) != nil {
		return "", ErrUnsafeConfigArtifact
	}
	return a, nil
}
func safeTarget(path string) (string, error) {
	if !safeLocalAbsolute(path) {
		return "", ErrUnsafeConfigArtifact
	}
	a, e := filepath.Abs(path)
	if e != nil || filepath.Clean(a) != a {
		return "", ErrUnsafeConfigArtifact
	}
	if validatePathHierarchy(a, false) != nil {
		return "", ErrUnsafeConfigArtifact
	}
	return a, nil
}
func safeLocalAbsolute(path string) bool {
	if len(path) < 4 || !filepath.IsAbs(path) || strings.ContainsRune(path, 0) || strings.Contains(path, "/") {
		return false
	}
	volume := filepath.VolumeName(path)
	if len(volume) != 2 || volume[1] != ':' || !asciiLetter(volume[0]) || len(path) <= len(volume) || path[len(volume)] != '\\' || filepath.Clean(path) != path {
		return false
	}
	for _, part := range strings.Split(path[len(volume)+1:], `\`) {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") || strings.ContainsAny(part, `<>:"|?*`) || reservedWindowsName(part) {
			return false
		}
	}
	return true
}

func asciiLetter(b byte) bool { return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' }
func reservedWindowsName(part string) bool {
	name := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
	if name == "CON" || name == "PRN" || name == "AUX" || name == "NUL" || name == "CLOCK$" {
		return true
	}
	return len(name) == 4 && (strings.HasPrefix(name, "COM") || strings.HasPrefix(name, "LPT")) && name[3] >= '1' && name[3] <= '9'
}
func existsSafe(path string) bool {
	i, e := os.Lstat(path)
	return e == nil && i.Mode()&os.ModeSymlink == 0 && !isReparse(path)
}
func resourceID(path string) string {
	s := sha256.Sum256([]byte(strings.ToLower(path)))
	return hex.EncodeToString(s[:])
}
func hash(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func randomID() (string, error) {
	b := make([]byte, 16)
	_, e := io.ReadFull(rand.Reader, b)
	return hex.EncodeToString(b), e
}
func writeVerifiedExclusive(path string, data []byte) error {
	expected := hash(data)
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return ErrPersistence
	}
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(path)
		}
	}()
	remaining := data
	for len(remaining) > 0 {
		n, e := f.Write(remaining)
		if e != nil || n == 0 {
			return ErrPersistence
		}
		remaining = remaining[n:]
	}
	if f.Sync() != nil || f.Close() != nil {
		return ErrPersistence
	}
	read, e := os.ReadFile(path)
	if e != nil || hash(read) != expected {
		return ErrPersistence
	}
	ok = true
	return nil
}
func fileHashEquals(path, want string) bool {
	b, e := os.ReadFile(path)
	return e == nil && hash(b) == want
}
func parseFailed(b []byte) bool { _, e := parseDocument(b); return e != nil }
func sortedKeys(m map[string]value) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func semanticMatches(have, want map[string]value) bool {
	for k, v := range want {
		h, ok := have[k]
		if !v.exists {
			if ok {
				return false
			}
			continue
		}
		if !ok || !equalValue(h, v) {
			return false
		}
	}
	return true
}
func makeRecords(target, identity string, meta documentMeta, pre, post, backup, backupHash string, original, applied map[string]value, at, version string, status state.RollbackStatus) []state.OwnershipRecord {
	var out []state.OwnershipRecord
	for _, k := range sortedKeys(applied) {
		o := original[k]
		out = append(out, state.OwnershipRecord{TargetPath: target, Format: state.FormatTOML, Encoding: state.EncodingUTF8, BOM: map[bool]state.BOM{true: state.BOMPresent, false: state.BOMAbsent}[meta.bom], Newline: map[bool]state.Newline{true: state.NewlineCRLF, false: state.NewlineLF}[meta.newline == "\r\n"], FileIdentity: identity, PreWriteHash: pre, KeyPath: k, OriginalExisted: o.exists, OriginalValue: toTyped(o), AppliedValue: toTyped(applied[k]), BackupPath: backup, BackupHash: backupHash, AppliedAt: at, ToolVersion: version, PostWriteHash: post, RollbackStatus: status})
	}
	return out
}
func toTyped(v value) state.TypedValue {
	if !v.exists {
		return state.TypedValue{Kind: state.ValueAbsent}
	}
	if v.table != nil {
		return state.TypedValue{Kind: state.ValueStringMap, StringMap: v.table}
	}
	s := v.str
	return state.TypedValue{Kind: state.ValueString, String: &s}
}
func fromTyped(v state.TypedValue) value {
	if v.Kind == state.ValueAbsent {
		return absentValue()
	}
	if v.Kind == state.ValueStringMap {
		return mapValue(v.StringMap)
	}
	if v.String != nil {
		return stringValue(*v.String)
	}
	return absentValue()
}
func recordsFor(o state.Ownership, target string) []state.OwnershipRecord {
	var out []state.OwnershipRecord
	for _, r := range o.Records {
		if strings.EqualFold(r.TargetPath, target) {
			out = append(out, r)
		}
	}
	return out
}
func valuesFromRecords(records []state.OwnershipRecord, original bool) map[string]value {
	m := map[string]value{}
	for _, r := range records {
		if original {
			m[r.KeyPath] = fromTyped(r.OriginalValue)
		} else {
			m[r.KeyPath] = fromTyped(r.AppliedValue)
		}
	}
	return m
}
func setStatus(o *state.Ownership, target string, from, to state.RollbackStatus) {
	for i := range o.Records {
		if strings.EqualFold(o.Records[i].TargetPath, target) && o.Records[i].RollbackStatus == from {
			o.Records[i].RollbackStatus = to
		}
	}
}
func removeRecords(records []state.OwnershipRecord, target string) []state.OwnershipRecord {
	out := records[:0]
	for _, r := range records {
		if !strings.EqualFold(r.TargetPath, target) {
			out = append(out, r)
		}
	}
	return out
}
func recordKeys(records []state.OwnershipRecord) []string {
	var k []string
	for _, r := range records {
		k = append(k, r.KeyPath)
	}
	sort.Strings(k)
	return k
}
func markerMatchesRecords(m marker, records []state.OwnershipRecord) bool {
	if len(records) != len(m.OwnedKeyPaths) {
		return false
	}
	keys := recordKeys(records)
	for i := range keys {
		if keys[i] != m.OwnedKeyPaths[i] {
			return false
		}
	}
	for _, r := range records {
		if m.Operation == "apply" {
			if r.PreWriteHash != m.PreHash || r.PostWriteHash != m.PostHash || filepath.Base(r.BackupPath) != m.BackupBasename || r.BackupHash != m.BackupHash || (r.RollbackStatus != state.RollbackPending && r.RollbackStatus != state.RollbackApplied) {
				return false
			}
		} else if r.RollbackStatus != state.RollbackApplied && r.RollbackStatus != state.RollbackRolledBack {
			return false
		}
	}
	return true
}
