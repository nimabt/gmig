package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"

	"github.com/emicklei/gmig"
	"github.com/urfave/cli"
)

const lastMigrationObject = "gmig-last-migration"

// space is right after timestamp
const logseparator = "~-------------- ---------------------~"

func cmdCreateMigration(c *cli.Context) error {
	verbose = c.Bool("v")
	desc := c.Args().First()
	filename := gmig.NewFilename(desc)
	m := gmig.Migration{
		Description: desc,
		DoSection:   []string{"gcloud config list"},
		UndoSection: []string{"gcloud config list"},
	}
	yaml, err := m.ToYAML()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, []byte(yaml), os.ModePerm)
}

func cmdMigrationsUp(c *cli.Context) error {
	lastApplied, _ := getStateProvider(c).LoadState()
	all, err := gmig.LoadMigrationsBetweenAnd(lastApplied, c.Args().First())
	if err != nil {
		return err
	}
	for _, each := range all {
		log.Println(logseparator)
		log.Println(each.Filename)
		log.Println(logseparator)
		if err := gmig.ExecuteAll(each.DoSection); err != nil {
			return err
		}
		lastApplied = each.Filename
		// save after each succesful migration
		if err := getStateProvider(c).SaveState(lastApplied); err != nil {
			return err
		}
	}
	return nil
}

func cmdMigrationsDown(c *cli.Context) error {
	lastApplied, _ := getStateProvider(c).LoadState()
	all, err := gmig.LoadMigrationsBetweenAnd("", lastApplied)
	if err != nil {
		return err
	}
	lastMigration := all[len(all)-1]
	log.Println(logseparator)
	log.Println(lastApplied)
	log.Println(logseparator)
	if err := gmig.ExecuteAll(lastMigration.UndoSection); err != nil {
		return err
	}
	// save after succesful migration
	previousFilename := ""
	if len(all) > 1 {
		previousFilename = all[len(all)-2].Filename
	}
	if err := getStateProvider(c).SaveState(previousFilename); err != nil {
		return err
	}
	return nil
}

func cmdMigrationsStatus(c *cli.Context) error {
	lastApplied, _ := getStateProvider(c).LoadState()
	all, err := gmig.LoadMigrationsBetweenAnd("", "")
	if err != nil {
		return err
	}
	log.Println(logseparator)
	var last string
	for _, each := range all {
		status := "--- applied ---"
		if each.Filename > lastApplied {
			status = "... pending ..."
			if len(last) > 0 && last != status {
				log.Println(logseparator)
			}
		}
		log.Println(status, each.Filename)
		last = status
	}
	log.Println(logseparator)
	return nil
}

func cmdInit(c *cli.Context) error {
	_, err := os.Stat(gmig.ConfigFilename)
	if err == nil {
		log.Println("configuration file [", gmig.ConfigFilename, "] already present")
		return nil
	}
	cfg := gmig.Config{
		Bucket:      "<your accessible bucket name>",
		StateObject: lastMigrationObject,
		Verbose:     false,
	}
	data, _ := json.Marshal(cfg)
	return ioutil.WriteFile(gmig.ConfigFilename, data, os.ModePerm)
}

func getStateProvider(c *cli.Context) gmig.StateProvider {
	verbose = c.GlobalBool("v")
	if verbose {
		log.Println("loading configuration from", gmig.ConfigFilename)
	}
	cfg, err := gmig.LoadConfig()
	if err != nil {
		log.Fatalln("error loading configuration (did you init?)", err)
	}
	cfg.Verbose = verbose
	return gmig.GCS{Configuration: cfg}
}