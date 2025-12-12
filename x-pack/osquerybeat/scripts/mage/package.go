// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package mage

import (
	"os"
	"path/filepath"

	devtools "github.com/elastic/beats/v7/dev-tools/mage"
	"github.com/elastic/beats/v7/x-pack/osquerybeat/internal/distro"
)

const defaultArch = "amd64"

// CustomizePackaging adds the files required for osquerybeat to packaging.
func CustomizePackaging() {
	for _, args := range devtools.Packages {
		arch := defaultArch
		if args.Arch != "" {
			arch = args.Arch
		}
		files := GetFilesForOSArch(args.OS, arch)
		for name, file := range files {
			args.Spec.Files[name] = devtools.PackageFile{
				Mode:         file.Mode,
				Source:       file.Source,
				PreserveMode: file.PreserveMode,
			}
		}
	}
}

// File is a generic structure returned from GetFilesForOSArch
type File struct {
	Mode         os.FileMode
	Source       string
	PreserveMode bool
}

// GetFilesForOSArch returns the needed files based on the os/arch combination.
func GetFilesForOSArch(osStr string, archStr string) map[string]File {
	files := make(map[string]File)
	distFile := distro.OsquerydDistroPlatformFilename(osStr)

	// The minimal change to fix the issue for 7.13
	// https://github.com/elastic/beats/issues/25762
	var mode os.FileMode = 0644
	// If distFile is osqueryd binary then it should be executable
	if distFile == distro.OsquerydFilenameForOS(osStr) {
		mode = 0750
	}
	packFile := File{
		Mode:   mode,
		Source: filepath.Join(distro.GetDataInstallDir(distro.OSArch{OS: osStr, Arch: archStr}), distFile),
	}

	// If macOS bundle osquery.app, preserve the directories and files permissions
	if distFile == distro.OsquerydDarwinApp() {
		packFile.PreserveMode = true
	}

	files[distFile] = packFile

	// Certs
	certsFile := File{
		Mode:   0640,
		Source: filepath.Join(distro.GetDataInstallDir(distro.OSArch{OS: osStr, Arch: archStr}), "certs", "certs.pem"),
	}

	files[filepath.Join("certs", "certs.pem")] = certsFile

	// Augeas lenses are not available for Windows
	if osStr != "windows" {
		files["lenses"] = File{Source: filepath.Join(distro.GetDataInstallDir(distro.OSArch{OS: osStr, Arch: archStr}), "lenses")}
	}
	return files
}
