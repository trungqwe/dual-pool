package app

import (
	"fmt"
	"io"

	"github.com/trungqwe/dual-pool/internal/apperr"
	"github.com/trungqwe/dual-pool/internal/buildinfo"
)

const helpText = `poolbridge

Usage:
  poolbridge version
  poolbridge help

Options:
  --version   Show build information
  --help, -h  Show this help
`

type App struct {
	build buildinfo.Info
}

func New(info buildinfo.Info) App { return App{build: info} }

func Run(args []string, stdout, stderr io.Writer) int {
	return New(buildinfo.Current()).Run(args, stdout, stderr)
}

func (application App) Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stdout, helpText)
		return apperr.CategorySuccess.ExitCode()
	}
	if len(args) != 1 {
		err := apperr.New(apperr.CodeInvalidArgument, nil)
		_, _ = fmt.Fprintln(stderr, err.Error())
		return err.Category().ExitCode()
	}

	switch args[0] {
	case "version", "--version":
		_, _ = io.WriteString(stdout, buildinfo.Format(application.build))
		return apperr.CategorySuccess.ExitCode()
	case "help", "--help", "-h":
		_, _ = io.WriteString(stdout, helpText)
		return apperr.CategorySuccess.ExitCode()
	default:
		err := apperr.New(apperr.CodeInvalidCommand, nil)
		_, _ = fmt.Fprintln(stderr, err.Error())
		return err.Category().ExitCode()
	}
}
