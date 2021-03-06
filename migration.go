package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-yaml/yaml"
)

// Migration holds shell commands for applying or reverting a change.
type Migration struct {
	Filename    string   `yaml:"-"`
	Description string   `yaml:"-"`
	DoSection   []string `yaml:"do"`
	UndoSection []string `yaml:"undo"`
}

// for testing
var timeNow = time.Now

// NewFilename generates a filename for storing a new migration.
func NewFilename(desc string) string {
	now := timeNow()
	sanitized := strings.Replace(strings.ToLower(desc), " ", "_", -1)
	return fmt.Sprintf("%d%02d%02dt%02d%02d%02d_%s.yaml", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second(), sanitized)
}

// LoadMigration reads and parses a migration from a named file.
func LoadMigration(filename string) (m Migration, err error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return m, err
	}
	m.Filename = filepath.Base(filename)
	err = yaml.Unmarshal(data, &m)
	return
}

// ToYAML returns the contents of a YAML encoded fixture.
func (m Migration) ToYAML() ([]byte, error) {
	out := new(bytes.Buffer)
	fmt.Fprintf(out, "# %s\n\n", m.Description)
	data, err := yaml.Marshal(m)
	if err != nil {
		return data, err
	}
	out.Write(data)
	return out.Bytes(), nil
}

// ExecuteAll the commands for this migration.
func ExecuteAll(commands []string, envs []string) error {
	if len(commands) == 0 {
		return nil
	}
	for i, each := range commands {
		log.Println(each)
		cmd := exec.Command("sh", "-c", each)
		cmd.Env = append(os.Environ(), envs...) // extend, not replace
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("%d: failed to run :%v", i, err)
		}
	}
	return nil
}

// LoadMigrationsBetweenAnd returns a list of pending Migration <firstFilename..lastFilename]
func LoadMigrationsBetweenAnd(workdir, firstFilename, lastFilename string) (list []Migration, err error) {
	// collect all filenames
	filenames := []string{}
	// firstFilename and lastFilename are relative to workdir.
	here, _ := os.Getwd()
	// change and restore finally
	os.Chdir(workdir)
	defer os.Chdir(here)
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if info.IsDir() || !isYamlFile(path) {
			return nil
		}
		filenames = append(filenames, path)
		return nil
	})
	// old -> new
	sort.StringSlice(filenames).Sort()
	// load only pending migrations
	for _, each := range filenames {
		// do not include firstFilename
		if each <= firstFilename {
			continue
		}
		var m Migration
		m, err = LoadMigration(filepath.Join(workdir, each))
		if err != nil {
			return
		}
		list = append(list, m)
		// include lastFilename
		if len(lastFilename) == 0 {
			continue
		}
		if each == lastFilename {
			return
		}
	}
	return
}
