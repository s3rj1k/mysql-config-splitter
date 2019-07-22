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
	cmdNewConfigSuffix string
)

// nolint: gochecknoinits
func init() {
	// read command flags
	flag.StringVar(&cmdConfigPath, "config", "/etc/mysql/my.cnf", "path to 'my.cnf'")
	flag.StringVar(&cmdNewConfigSuffix, "suffix", ".new", "suffix for newly created 'my.cnf'")

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

func createKeyFile(name, value, dir string) error {
	// create include dir
	if err := os.MkdirAll(dir, 0775); err != nil {
		return err
	}

	// write key=value key.cnf
	return ioutil.WriteFile(
		filepath.Join(dir, name+".cnf"),
		[]byte(fmt.Sprintf("%s = %s\n", name, value)),
		0664,
	)
}

func main() {
	config, err := readMySQLConfig(cmdConfigPath)
	if err != nil {
		log.Fatalf("failed to read 'my.cnf' from: '%s'", cmdConfigPath)
	}

	// contains lines for new 'my.cnf'
	newConfigContent := make([]string, 0)
	// path to old 'my.cnf'
	baseDir := filepath.Dir(cmdConfigPath)

	// loop-over config file sections
	for _, section := range config.Sections() {
		// compute new include directory path based on section name and base dir
		includeDir := filepath.Join(baseDir, section.Name()+".d")
		// append include rules to new config
		newConfigContent = append(newConfigContent, fmt.Sprintf("[%s]\n!includedir %s\n", section.Name(), includeDir))
		// loop-over keys in current config section
		for _, key := range section.Keys() {
			// create key file
			if err := createKeyFile(key.Name(), key.Value(), includeDir); err != nil {
				log.Fatal(err)
			}
		}
	}

	// create new config file
	if err := ioutil.WriteFile(cmdConfigPath+cmdNewConfigSuffix, []byte(strings.Join(newConfigContent, "\n")), 0664); err != nil {
		log.Fatal(err)
	}
}
