/*
 Copyright 2020 Qiniu Cloud (qiniu.com)

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/qiniu/goc/pkg/cover"
	log "github.com/sirupsen/logrus"
)

// Build is to describe the building/installing process of a goc build/install
type Build struct {
	Pkgs          map[string]*cover.Package // Pkg list parsed from "go list -json ./..." command
	NewGOPATH     string                    // the new GOPATH
	OriGOPATH     string                    // the original GOPATH
	TmpDir        string                    // the temporary directory to build the project
	TmpWorkingDir string                    // the working directory in the temporary directory, which is corresponding to the current directory in the project directory
	IsMod         bool                      // determine whether it is a Mod project
	Root          string
	// go 1.11, go 1.12 has no Root
	// Project Root:
	// 1. legacy, root == GOPATH
	// 2. mod, root == go.mod Dir
	Target string // the binary name that go build generate
	// keep compatible with go commands:
	// go run [build flags] [-exec xprog] package [arguments...]
	// go build [-o output] [-i] [build flags] [packages]
	// go install [-i] [build flags] [packages]
	BuildFlags     string // Build flags
	Packages       string // Packages that needs to build
	GoRunExecFlag  string // for the -exec flags in go run command
	GoRunArguments string // for the '[arguments]' parameters in go run command
}

// NewBuild creates a Build struct which can build from goc temporary directory,
// and generate binary in current working directory
func NewBuild(buildflags string, packages string, outputDir string) (*Build, error) {
	// buildflags = buildflags + " -o " + outputDir
	b := &Build{
		BuildFlags: buildflags,
		Packages:   packages,
	}
	if false == b.validatePackageForBuild() {
		log.Errorln(ErrWrongPackageTypeForBuild)
		return nil, ErrWrongPackageTypeForBuild
	}
	b.MvProjectsToTmp()
	dir, err := b.determineOutputDir(outputDir)
	b.Target = dir
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (b *Build) Build() error {
	log.Infoln("Go building in temp...")
	// new -o will overwrite  previous ones
	b.BuildFlags = b.BuildFlags + " -o " + b.Target
	cmd := exec.Command("/bin/bash", "-c", "go build "+b.BuildFlags+" "+b.Packages)
	cmd.Dir = b.TmpWorkingDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if b.NewGOPATH != "" {
		// Change to temp GOPATH for go install command
		cmd.Env = append(os.Environ(), fmt.Sprintf("GOPATH=%v", b.NewGOPATH))
	}

	log.Printf("go build cmd is: %v", cmd.Args)
	err := cmd.Start()
	if err != nil {
		log.Errorf("Fail to execute: %v. The error is: %v", cmd.Args, err)
		return fmt.Errorf("fail to execute: %v: %w", cmd.Args, err)
	}
	if err = cmd.Wait(); err != nil {
		log.Errorf("go build failed. The error is: %v", err)
		return fmt.Errorf("go build faileds: %w", err)
	}
	log.Println("Go build exit successful.")
	return nil
}

// determineOutputDir, as we only allow . as package name,
// the binary name is always same as the directory name of current directory
func (b *Build) determineOutputDir(outputDir string) (string, error) {
	if b.TmpDir == "" {
		log.Errorf("Can only be called after Build.MvProjectsToTmp(): %v", ErrWrongCallSequence)
		return "", fmt.Errorf("can only be called after Build.MvProjectsToTmp(): %w", ErrWrongCallSequence)
	}
	curWorkingDir, err := os.Getwd()
	if err != nil {
		log.Errorf("Cannot get current working directory: %v", err)
		return "", fmt.Errorf("cannot get current working directory: %w", err)
	}

	if outputDir == "" {
		_, last := filepath.Split(curWorkingDir)
		if b.IsMod {
			// in mod, special rule
			// replace "_" with "-" in the import path
			last = strings.ReplaceAll(last, "_", "-")
		}
		return filepath.Join(curWorkingDir, last), nil
	}
	abs, err := filepath.Abs(outputDir)
	if err != nil {
		log.Errorf("Fail to transform the path: %v to absolute path: %v", outputDir, err)
		return "", fmt.Errorf("fail to transform the path %v to absolute path: %w", outputDir, err)
	}
	return abs, nil
}

// validatePackageForBuild only allow . as package name
func (b *Build) validatePackageForBuild() bool {
	if b.Packages == "." {
		return true
	}
	return false
}

// Run excutes the main package in addition with the internal goc features
func (b *Build) Run() {
	cmd := exec.Command("/bin/bash", "-c", "go run "+b.BuildFlags+" "+b.GoRunExecFlag+" "+b.Packages+" "+b.GoRunArguments)
	cmd.Dir = b.TmpWorkingDir

	if b.NewGOPATH != "" {
		// Change to temp GOPATH for go install command
		cmd.Env = append(os.Environ(), fmt.Sprintf("GOPATH=%v", b.NewGOPATH))
	}

	log.Printf("go build cmd is: %v", cmd.Args)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		log.Fatalf("Fail to start command: %v. The error is: %v", cmd.Args, err)
	}

	if err = cmd.Wait(); err != nil {
		log.Fatalf("Fail to execute command: %v. The error is: %v", cmd.Args, err)
	}

}