package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

// nolint: gochecknoglobals
var (
	cmdConfigPath      string
	cmdIncludeDir      string
	cmdNewConfigSuffix string
	cmdCleanIncludeDir bool
)

// nolint: gochecknoinits
func init() {
	// read command flags
	flag.StringVar(&cmdConfigPath, "config", "/etc/mysql/my.cnf", "path to config, 'my.cnf'")
	flag.StringVar(&cmdIncludeDir, "include-dir", "/etc/mysql/conf.d", "path to config include directory, 'conf.d'")
	flag.BoolVar(&cmdCleanIncludeDir, "clean-include-dir", false, "clean include directory beforehand")
	flag.StringVar(&cmdNewConfigSuffix, "suffix", ".new", "suffix for newly created config, 'my.cnf'")

	flag.Parse()
}

func readMySQLConfig(path string) (*ini.File, error) {
	// parse 'my.cnf' from defined path
	config, err := ini.LoadSources(
		ini.LoadOptions{
			Insensitive:             false,
			IgnoreInlineComment:     false,
			AllowBooleanKeys:        true,
			SkipUnrecognizableLines: true,
			AllowShadows:            false,
			KeyValueDelimiters:      "=",
			PreserveSurroundedQuote: true,
		},
		path,
	)
	if err != nil {
		return nil, err
	}

	// in 'my.cnf' there is no default section
	config.DeleteSection(ini.DefaultSection)

	// loop-over all sections in 'my.cnf'
	for _, section := range config.Sections() {
		// loop-over all keys in section
		for _, key := range section.Keys() {
			// check that key name contains old "-" naming
			if strings.Contains(key.Name(), "-") {
				// create newer name using "_"
				newKeyName := strings.Replace(key.Name(), "-", "_", -1)
				// store key value
				newKeyValue := key.Value()
				// delete older key, with '-'
				section.DeleteKey(key.Name())
				// check thatnewer key does not exists in section
				if !section.HasKey(newKeyName) {
					if len(newKeyValue) == 0 { // then value zero-sized, this is boolean key, create newer key
						if _, err := section.NewBooleanKey(newKeyName); err != nil {
							return nil, err
						}
					} else { // create key with value
						if _, err := section.NewKey(newKeyName, newKeyValue); err != nil {
							return nil, err
						}
					}
				}
			}

			// check that key name is '!includedir'
			if strings.HasPrefix(key.Name(), "!includedir") {
				// delete this key
				section.DeleteKey(key.Name())
			}

			// check that key name is '!include'
			if strings.HasPrefix(key.Name(), "!include") {
				// delete this key
				section.DeleteKey(key.Name())
			}
		}
	}

	return config, nil
}

func main() {
	config, err := readMySQLConfig(cmdConfigPath)
	if err != nil {
		log.Fatalf("failed to read 'my.cnf' from: '%s'", cmdConfigPath)
	}

	if cmdCleanIncludeDir {
		// clean include directory
		if err := os.RemoveAll("/tmp/"); err != nil {
			log.Fatal(err)
		}
	}

	// create include directory
	if err := os.MkdirAll(cmdIncludeDir, 0775); err != nil {
		log.Fatal(err)
	}

	// loop-over config file sections
	for _, section := range config.Sections() {
		// loop-over keys in current config section
		for _, key := range section.Keys() {
			// prepare content for key=value config
			content := fmt.Sprintf("[%s]\n%s = %s\n", section.Name(), key.Name(), key.Value())
			// write key=value key.cnf
			if err := ioutil.WriteFile(filepath.Join(cmdIncludeDir, key.Name()+".cnf"), []byte(content), 0664); err != nil {
				log.Fatal(err)
			}
		}
	}

	// create new config file
	if err := ioutil.WriteFile(
		cmdConfigPath+cmdNewConfigSuffix,
		[]byte(fmt.Sprintf("!includedir %s\n", cmdIncludeDir)),
		0664,
	); err != nil {
		log.Fatal(err)
	}
}
