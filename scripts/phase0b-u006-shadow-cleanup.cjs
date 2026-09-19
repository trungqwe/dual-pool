'use strict';
const mod = require('./phase0b-u006-shadow-home-watchdog.cjs');
const root = process.argv[2];
if (!root || !mod.safeSessionRoot(root)) throw new Error('UNSAFE_SHADOW_ROOT');
if (mod.getAntigravityProcessCount() !== 0) throw new Error('PROBE_STILL_RUNNING');
if (!mod.cleanupShadow(root)) throw new Error('SHADOW_CLEANUP_FAILED');
console.log('SHADOW_CLEANUP_PASS');
