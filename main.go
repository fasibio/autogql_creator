package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"syscall"

	"github.com/urfave/cli/v3"

	_ "embed"
)

//go:embed doc.txt
var docFile string

//go:embed tools.go.tmp
var toolsGoFile []byte

//go:embed pluginFile.go.tmp
var pluginMainGoFile []byte

//go:embed schemaGraphQL.tmp
var schemaFile []byte

//go:embed resolver.go.tmp
var resolverGoFile []byte

//go:embed gqlgenyml.tmp
var gqlgenymlFile []byte

//go:embed server.go.tmp
var serverGoFile []byte

func main() {
	ctx := context.Background()

	runner := Runner{
		Cfg: &Config{},
	}
	app := &cli.Command{
		Usage: "autogql creator",
		Commands: []*cli.Command{
			{
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:        "root path where creating the autogql project",
						Destination: &runner.Cfg.Path,
					},
				},
				Name: "init",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "gopath",
						Value:       "",
						Usage:       "to test gopath",
						Destination: &runner.Cfg.GoPath,
					},
				},
				Usage:  "to create a new autogql project",
				Action: runner.Create,
			},
		},
	}
	if err := app.Run(ctx, os.Args); err != nil {
		fmt.Printf("an error occurred: %s\n", err)
	}
}

type Config struct {
	Path   string
	GoPath string
}

type Runner struct {
	Cfg *Config
}

func (r *Runner) checkPathIsDirEmptyOrNotExist() error {
	info, err := os.Stat(string(r.Cfg.Path))
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.Mkdir(string(r.Cfg.Path), 0755); err != nil {
				return err
			}
			i, err := os.Stat(string(r.Cfg.Path))
			info = i
			if err != nil {
				return fmt.Errorf("unable to get info over path %w", err)
			}
		} else {
			return fmt.Errorf("unable to get info over path %w", err)
		}

	}

	if !info.IsDir() {
		return fmt.Errorf("path have to be an folder no file")
	}
	return nil
}

func (r *Runner) ExecuteAtFolder(ctx context.Context, name string, arg ...string) error {
	return r.ExecuteAtFolderWithMap(ctx, true, name, arg...)
}

func (r *Runner) ExecuteAtFolderWithMap(ctx context.Context, mapPrints bool, name string, arg ...string) error {
	cmd := exec.CommandContext(ctx, name, arg...)
	go func() {
		<-ctx.Done()
		// Kill the whole process group
		if cmd.Process != nil {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) //nolint:errcheck //there is nothing we can do when an error is happen here
		}
	}()
	cmd.Dir = string(r.Cfg.Path)
	if mapPrints {
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // create new process group
	}
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("command %s %v returns error: %w", name, arg, err)
	}
	return nil
}

func (r *Runner) ExecuteTidy(ctx context.Context) error {
	return r.ExecuteAtFolder(ctx, "go", "mod", "tidy")
}

func (r *Runner) Create(ctx context.Context, c *cli.Command) error {
	if err := r.checkPathIsDirEmptyOrNotExist(); err != nil {
		return err
	}
	if err := r.ExecuteAtFolder(ctx, "go", "mod", "init", r.Cfg.GoPath); err != nil {
		return err
	}
	if err := os.WriteFile(path.Join(r.Cfg.Path, "tools.go"), toolsGoFile, 0644); err != nil {
		return err
	}
	if err := r.ExecuteTidy(ctx); err != nil {
		return err
	}
	if err := r.ExecuteAtFolder(ctx, "go", "run", "github.com/99designs/gqlgen", "init"); err != nil {
		return err
	}
	if err := r.ExecuteTidy(ctx); err != nil {
		return err
	}

	if err := r.ExecuteAtFolder(ctx, "git", "init"); err != nil {
		return err
	}

	if err := r.ExecuteAtFolder(ctx, "git", "add", "."); err != nil {
		return err
	}

	if err := r.ExecuteAtFolder(ctx, "git", "commit", "-m", "gqlgen init finished"); err != nil {
		return err
	}

	if err := os.Mkdir(path.Join(r.Cfg.Path, "plugin"), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path.Join(r.Cfg.Path, "plugin", "main.go"), pluginMainGoFile, 0644); err != nil {
		return err
	}
	if err := r.ExecuteTidy(ctx); err != nil {
		return err
	}
	schemaFilePath := path.Join(r.Cfg.Path, "graph", "schema.graphqls")
	if err := os.Remove(schemaFilePath); err != nil {
		return err
	}
	if err := os.WriteFile(schemaFilePath, schemaFile, 0644); err != nil {
		return err
	}

	gqlgenYamlPath := path.Join(r.Cfg.Path, "gqlgen.yml")

	if err := os.Remove(gqlgenYamlPath); err != nil {
		return err
	}
	if err := os.WriteFile(gqlgenYamlPath, gqlgenymlFile, 0644); err != nil {
		return err
	}

	r.ExecuteAtFolderWithMap(ctx, false, "go", "run", "plugin/main.go") //nolint:errcheck //we know it will cotains but this is fine at the pipe

	if err := r.ExecuteTidy(ctx); err != nil {
		return err
	}

	resolver := path.Join(r.Cfg.Path, "graph", "resolver.go")
	if err := os.Remove(resolver); err != nil {
		return err
	}
	if err := os.WriteFile(resolver, resolverGoFile, 0644); err != nil {
		return err
	}

	if err := r.ExecuteAtFolder(ctx, "gopls", "imports", "-w", path.Join("graph/resolver.go")); err != nil {
		return err
	}

	serverPath := path.Join(r.Cfg.Path, "server.go")

	if err := os.Remove(serverPath); err != nil {
		return err
	}
	if err := os.WriteFile(serverPath, serverGoFile, 0644); err != nil {
		return err
	}

	if err := r.ExecuteTidy(ctx); err != nil {
		return err
	}

	if err := r.ExecuteAtFolder(ctx, "gopls", "imports", "-w", path.Join("server.go")); err != nil {
		return err
	}
	fmt.Print(docFile)
	fmt.Printf("to execute:\ncd %s \ngo run server.go\n", r.Cfg.Path)

	return nil
}
