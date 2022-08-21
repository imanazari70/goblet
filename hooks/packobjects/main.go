// Copyright 2022 Canva Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
)

// return the stdin buffer, stdin size and the key (=sha256sum(args+stdin))
func obtainStdinAndKey(cmd *exec.Cmd) (*bytes.Buffer, int64, string) {
	skey := sha256.New()

	args := strings.TrimSpace(strings.Join(cmd.Args, " "))
	fmt.Fprintln(skey, args)

	var bstdin bytes.Buffer
	stdin_size, _ := io.Copy(io.MultiWriter(&bstdin, skey), os.Stdin)

	key := hex.EncodeToString(skey.Sum(nil))

	return &bstdin, stdin_size, key
}

func fileExists(name string) bool {
	_, err := os.Stat(name)
	return !errors.Is(err, os.ErrNotExist)
}

func cachePath(dir, key string, file ...string) string {
	parts := []string{dir, key[:2], key[2:]}
	parts = append(parts, file...)
	return path.Join(parts...)
}

// prepare the cache dir, if successful, return the writer for stdout
func prepareCacheDir(dir, key string, stdin_size, stdin_max int64) (io.WriteCloser, bool) {
	if stdin_max != 0 && stdin_size > stdin_max {
		return nil, false
	}

	if fileExists(cachePath(dir, key, "start")) {
		return nil, false
	}

	err := os.MkdirAll(cachePath(dir, key), 0750)
	if err != nil && errors.Is(err, os.ErrExist) {
		return nil, false
	}

	// https://stackoverflow.com/questions/33223564/atomically-creating-a-file-if-it-doesnt-exist-in-python
	// the "start" file effectively serves as a lock.
	fstart, err := os.OpenFile(cachePath(dir, key, "start"), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return nil, false
	}
	fmt.Fprintln(fstart, time.Now().Format(time.RFC3339))
	fstart.Close()

	fstdout, err := os.OpenFile(cachePath(dir, key, "stdout"), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, false
	}
	return fstdout, true
}

func purgeCacheDir(dir, key string) {
	// remove "end" first. So the cache won't be used.
	os.RemoveAll(cachePath(dir, key, "end"))

	// remove "stdout" so nothing will be served
	os.RemoveAll(cachePath(dir, key, "stdout"))

	// remove everything else
	os.RemoveAll(cachePath(dir, key))
}

func main() {
	// dont print out any extra information
	log.SetFlags(0)

	args := os.Args[1:]
	if len(args) == 0 {
		log.Fatalln("No command to execute.")
	}

	dir, ok := os.LookupEnv("POH_CACHE_DIR")
	if !ok {
		log.Fatalln("POH_CACHE_DIR not set.")
	}

	ci_source := os.Getenv("POH_CI_SOURCE")

	var stdin_max int64
	if v, err := strconv.ParseInt(os.Getenv("POH_STDIN_MAX"), 10, 0); err == nil {
		stdin_max = v
	}

	if err := os.MkdirAll(dir, 0750); err != nil {
		log.Fatalf("Failed to create POH_CACHE_DIR (%s).\n", dir)
	}

	var exitCode int
	defer func() {
		recover()
		os.Exit(exitCode)
	}()

	cmd := exec.Command(args[0], args[1:]...)
	stdin, stdin_size, key := obtainStdinAndKey(cmd)
	cmd.Stdin = stdin
	cmd.Stderr = os.Stderr

	if fileExists(cachePath(dir, key, "end")) {
		if fstdout, err := os.Open(cachePath(dir, key, "stdout")); err == nil {
			_, err := io.Copy(os.Stdout, fstdout)
			fstdout.Close()

			if err != nil {
				exitCode = 1
			}

			// append `ci_source` to `served`
			if fserved, err := os.OpenFile(cachePath(dir, key, "served"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				fmt.Fprintln(fserved, time.Now().Format(time.RFC3339), ci_source, exitCode)
				fserved.Close()
			}

			return
		} else {
			// stdout is broken, purge it
			purgeCacheDir(dir, key)
		}
	}

	fstdout, writeCache := prepareCacheDir(dir, key, stdin_size, stdin_max)
	if writeCache {
		cmd.Stdout = io.MultiWriter(os.Stdout, fstdout)
	} else {
		cmd.Stdout = os.Stdout
	}

	runError := cmd.Run()
	if runError != nil {
		if exitError, ok := runError.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			log.Printf("Failed to execute %s. Error: %v\n", cmd, runError)
			exitCode = 1
		}
	} else {
		exitCode = 0
	}

	if writeCache {
		fstdout.Close()
		if exitCode == 0 {
			if fend, err := os.OpenFile(cachePath(dir, key, "end"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				fmt.Fprintln(fend, time.Now().Format(time.RFC3339))
				fend.Close()
			}
		} else {
			purgeCacheDir(dir, key)
		}
	}
}
