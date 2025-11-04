package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/urfave/cli/v3"

	_ "embed"
)

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
				Name:   "init",
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
	Path string
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

func (r *Runner) ExecuteAtFolder(name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	cmd.Dir = string(r.Cfg.Path)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("command %s %v returns error: %w", name, arg, err)
	}
	return nil
}

func (r *Runner) ExecuteTidy() error {
	return r.ExecuteAtFolder("go", "mod", "tidy")
}

func (r *Runner) Create(ctx context.Context, c *cli.Command) error {
	if err := r.checkPathIsDirEmptyOrNotExist(); err != nil {
		return err
	}
	if err := r.ExecuteAtFolder("go", "mod", "init"); err != nil {
		return err
	}
	if err := os.WriteFile(path.Join(r.Cfg.Path, "tools.go"), toolsGoFile, 0644); err != nil {
		return err
	}
	if err := r.ExecuteTidy(); err != nil {
		return err
	}
	if err := r.ExecuteAtFolder("go", "run", "github.com/99designs/gqlgen", "init"); err != nil {
		return err
	}
	if err := r.ExecuteTidy(); err != nil {
		return err
	}

	if err := r.ExecuteAtFolder("git", "init"); err != nil {
		return err
	}

	if err := r.ExecuteAtFolder("git", "add", "."); err != nil {
		return err
	}

	if err := r.ExecuteAtFolder("git", "commit", "-m", "gqlgen init finished"); err != nil {
		return err
	}

	if err := os.Mkdir(path.Join(r.Cfg.Path, "plugin"), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path.Join(r.Cfg.Path, "plugin", "main.go"), pluginMainGoFile, 0644); err != nil {
		return err
	}
	if err := r.ExecuteTidy(); err != nil {
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

	if err := r.ExecuteAtFolder("go", "run", "plugin/main.go"); err != nil {
		//no break error is fine

	}
	if err := r.ExecuteTidy(); err != nil {
		return err
	}

	resolver := path.Join(r.Cfg.Path, "graph", "resolver.go")
	if err := os.Remove(resolver); err != nil {
		return err
	}
	if err := os.WriteFile(resolver, resolverGoFile, 0644); err != nil {
		return err
	}

	if err := r.ExecuteAtFolder("gopls", "imports", "-w", path.Join("graph/resolver.go")); err != nil {
		return err
	}

	serverPath := path.Join(r.Cfg.Path, "server.go")

	if err := os.Remove(serverPath); err != nil {
		return err
	}
	if err := os.WriteFile(serverPath, serverGoFile, 0644); err != nil {
		return err
	}

	if err := r.ExecuteTidy(); err != nil {
		return err
	}

	if err := r.ExecuteAtFolder("gopls", "imports", "-w", path.Join("server.go")); err != nil {
		return err
	}
	fmt.Printf(`git init and git commit was executed between gqlgen setup and autogqlgen.
So you can check the changes. if something was removed what you want to hold.

You can update the schema at graph/schema.graphqls.
After changing run:
	go run plugin/main.go to regenerate code. 

As Default a SQLITE was used. 
You can change this at server.go

Some Queries/Mutation you can directly execute open browser: 

mutation addTodo {
  addTodo(input: { id: 1, text: "Start writing autogql", userID: 1, done: false }) {
    affected {
      id
      text
      createdAt
      done
    }
  }
}

query todos {
  queryTodo(filter: { user: { name: { startsWith: "Did" } } }) {
    data {
      id
      text
      user {
        name
      }
    }
  }
}

query showFirst {
  getUser(id: 1) {
    id
    name
    todos {
      text
      createdAt
      done
    }
  }
}

mutation addUser {
  addUser(
    input: { id: 1, name: "Did you see the autoincrement feature?", todos: [] }
  ) {
    affected {
      id
      name
    }
  }
}

mutation removeFirst {
  deleteUser(filter: { id: { eq: 1 } }) {
    count
  }
}`)
	fmt.Printf("to execute:\ncd %s \ngo run server.go\n", r.Cfg.Path)

	return nil
}
