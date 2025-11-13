package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/machinebox/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const addUserMut = `
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
`

const addTodoMut = `
mutation addTodo {                                                                           
	addTodo(input: { id: 1, text: "Start writing autogql", userID: 1, done: false }) {         
		affected {                                                                               
			id                                                                                     
			text                                                                                   
			done                                                                                   
		}                                                                                        
	}                                                                                          
}
`

const QueryTodos = `
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
}   `

func TestCreation(t *testing.T) {
	testFolder := "integration_test_output"
	defer func() {
		os.RemoveAll(testFolder) //nolint:errcheck //there is nothing we can do here
	}()
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()

	r := Runner{
		Cfg: &Config{
			Path:   testFolder,
			GoPath: fmt.Sprintf("github.com/fasibio/autogql_creator/%s", testFolder),
		},
	}

	err := r.Create(ctx, nil)
	assert.NoError(err)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		r.ExecuteAtFolder(runCtx, "go", "run", "server.go") //nolint:errcheck //there is nothing we can do here
	}()

	serverUrl := "http://localhost:8080"
	playgroundUrl, err := url.JoinPath(serverUrl, "playground")
	require.NoError(err)
	queryUrl, err := url.JoinPath(serverUrl, "query")
	require.NoError(err)
	require.Eventually(func() bool {

		res, err := http.Get(playgroundUrl)
		if err != nil {
			return false
		}
		if res.StatusCode != 200 {
			return false
		}
		return true
	}, 20*time.Second, 1*time.Second)

	client := graphql.NewClient(queryUrl)
	for _, r := range []string{addUserMut, addTodoMut, QueryTodos} {
		req := graphql.NewRequest(r)
		var res any
		require.NoError(client.Run(ctx, req, &res))
		snaps.MatchJSON(t, res)

	}
}
