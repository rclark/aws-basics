package command

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

func Pipe(ctx context.Context, src *Process, dst *Process) error {
	return src.Pipe(ctx, dst)
}

func Run(ctx context.Context, p *Process) error {
	return p.Run(ctx)
}

type Process struct {
	WorkingDirectory     string
	EnvironmentVariables []string
	Command              string
	Arguments            []string
}

func (p *Process) Run(ctx context.Context) error {
	all := []string{p.Command}
	all = append(all, p.Arguments...)
	full := strings.Join(all, " ")
	fmt.Printf("--> %s\n", full)

	cmd := exec.Command(p.Command, p.Arguments...)
	cmd.Dir = p.WorkingDirectory
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if p.EnvironmentVariables != nil {
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, p.EnvironmentVariables...)
	}

	errs := make(chan error)
	go func() {
		errs <- errors.Wrapf(cmd.Run(), `command "%s" failed`, full)
	}()

	select {
	case <-ctx.Done():
		msg := fmt.Sprintf(`command "%s" did not complete, context canceled`, full)
		if err := cmd.Process.Kill(); err != nil {
			return errors.Wrap(err, msg)
		}
		return errors.New(msg)

	case err := <-errs:
		return err
	}
}

func (p *Process) Pipe(ctx context.Context, to *Process) error {
	all := []string{p.Command}
	all = append(all, p.Arguments...)
	all = append(all, "|")
	all = append(all, to.Command)
	all = append(all, to.Arguments...)
	full := strings.Join(all, " ")
	fmt.Printf("--> %s\n", full)

	src := exec.Command(p.Command, p.Arguments...)
	src.Dir = p.WorkingDirectory
	src.Stderr = os.Stderr

	if p.EnvironmentVariables != nil {
		src.Env = os.Environ()
		src.Env = append(src.Env, p.EnvironmentVariables...)
	}

	dst := exec.Command(to.Command, to.Arguments...)
	dst.Dir = to.WorkingDirectory
	dst.Stderr = os.Stderr

	if to.EnvironmentVariables != nil {
		dst.Env = os.Environ()
		dst.Env = append(dst.Env, to.EnvironmentVariables...)
	}

	stdout, _ := src.StdoutPipe()
	stdin, _ := dst.StdinPipe()

	errs := make(chan error)
	go func() {
		if err := src.Start(); err != nil {
			errs <- errors.Wrapf(err, `command failed to start "%s %s"`, p.Command, strings.Join(p.Arguments, " "))
			return
		}

		if err := dst.Start(); err != nil {
			errs <- errors.Wrapf(err, `command failed to start "%s %s"`, to.Command, strings.Join(to.Arguments, " "))
			return
		}

		if _, err := io.Copy(stdin, stdout); err != nil {
			errs <- errors.Wrap(err, "failed to pipe between processes")
			return
		}

		if err := src.Wait(); err != nil {
			errs <- errors.Wrapf(err, `command failed to finish "%s %s"`, p.Command, strings.Join(p.Arguments, " "))
			return
		}

		if err := dst.Wait(); err != nil {
			errs <- errors.Wrapf(err, `command failed to finish "%s %s"`, to.Command, strings.Join(to.Arguments, " "))
			return
		}
	}()

	select {
	case <-ctx.Done():
		msg := "piped processes did not complete, context canceled"
		if err := src.Process.Kill(); err != nil {
			return errors.Wrap(err, msg)
		}
		if err := dst.Process.Kill(); err != nil {
			return errors.Wrap(err, msg)
		}
		return errors.New(msg)
	case err := <-errs:
		return errors.Wrap(err, "piped processes failed")
	}
}
