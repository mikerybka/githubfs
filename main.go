package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/mikerybka/util"
)

func main() {
	http.HandleFunc("/{orgID}/{repoID}", handle)
	http.HandleFunc("/{orgID}/{repoID}/{filePath...}", handle)
	err := http.ListenAndServe(":2070", nil)
	panic(err)
}

func handle(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgID")
	repoID := r.PathValue("repoID")
	filePath := r.PathValue("filePath")

	if r.Method == http.MethodGet {
		f, err := get(orgID, repoID, filePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		io.Copy(w, f)
		return
	}

	if r.Method == http.MethodPut {
		err := save(orgID, repoID, filePath, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	if r.Method == http.MethodDelete {
		err := del(orgID, repoID, filePath, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}
}

type DirEntry struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func get(org, repo, filePath string) (io.Reader, error) {
	err := sync(org, repo)
	if err != nil {
		return nil, err
	}
	workdir := util.RequireEnvVar("WORKDIR")
	path := filepath.Join(workdir, org, repo, filePath)
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		res := []DirEntry{}
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			t := "file"
			if e.IsDir() {
				t = "dir"
			}
			res = append(res, DirEntry{
				Name: e.Name(),
				Type: t,
			})
		}
		b, err := json.Marshal(res)
		if err != nil {
			panic(err)
		}
		return bytes.NewReader(b), nil
	}
	return os.Open(path)
}

func sync(org, repo string) error {
	workdir := util.RequireEnvVar("WORKDIR")
	if !exists(workdir, org, repo) {
		return clone(org, repo)
	} else {
		return pull(org, repo)
	}
}

func exists(path ...string) bool {
	_, err := os.Stat(filepath.Join(path...))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false
		}
		panic(err)
	}
	return true
}
