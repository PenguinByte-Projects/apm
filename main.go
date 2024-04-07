package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"io/ioutil"
	"os"
	"bufio"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

type PackageInfo struct {
	Name        string   `json:"name"`
	Dependencies    []string `json:"dependencies"`
	InstallScript   string   `json:"install_script"`
	InstalledPath   string   `json:"installed_path"`
	Version     string   `json:"version"`
}
type Repository struct {
    RemoteURL string `json:"remoteURL"`
    LocalPath string `json:"localPath"`
}
func readRepositories() ([]Repository, error) {
    file, err := os.Open("/packages/repos.json")
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var repositories []Repository
    if err := json.NewDecoder(file).Decode(&repositories); err != nil {
        return nil, err
    }

    return repositories, nil
}

func installPackage(packageName string, systemWide bool, userName string) {
	packageDir := filepath.Join("/packages/repos/", packageName)
		if _, err := os.Stat(packageDir); os.IsNotExist(err) {
		fmt.Printf("Package %s not found.\n", packageName)
		return
	}

	// Ensure the store directory exists
	storeDir := filepath.Join("/packages/store/", packageName)
	if _, err := os.Stat(storeDir); os.IsNotExist(err) {
		err := os.MkdirAll(storeDir, 0755) // 0755 sets the permissions for the directory
		if err != nil {
			fmt.Println("Error creating directory:", err)
			return
		}
	}

	// Copy package.json to the store directory
	src := filepath.Join(packageDir, "package.json")
	dst := filepath.Join(storeDir, "package.json")
	if err := copyFile(src, dst); err != nil {
		fmt.Println("Error copying package.json:", err)
		return
	}
	if _, err := os.Stat(packageDir); os.IsNotExist(err) {
		fmt.Printf("Package %s not found.\n", packageName)
		return
	}

	packageInfoBytes, err := ioutil.ReadFile(filepath.Join(packageDir, "package.json"))
	if err != nil {
		fmt.Println("Error reading package.json:", err)
		return
	}

	var packageInfo PackageInfo
	if err := json.Unmarshal(packageInfoBytes, &packageInfo); err != nil {
		fmt.Println("Error parsing package.json:", err)
		return
	}

for _, dependency := range packageInfo.Dependencies {
    // Assuming dependencies should be installed with the same scope as the parent package
    // If the parent installation is user-specific, install dependencies for the same user
    // If the parent installation is system-wide, install dependencies system-wide
    if userName != "" {
        installPackage(dependency, false, userName)
    } else {
        installPackage(dependency, true, "")
    }
}


	if packageInfo.InstallScript == "" {
		fmt.Printf("No install script specified for package %s.\n", packageName)
		return
	}

	// Assuming the install script is a shell script that can be executed directly
	// This is a simplification; in a real-world scenario, you might need to handle this differently
	scriptPath := filepath.Join(packageDir, packageInfo.InstallScript)
	cmd := exec.Command("sh", scriptPath, filepath.Join(packageDir, "package.json"), packageDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Installation of %s failed: %v\n", packageName, err)
		return
	}

 
    if packageInfo.InstalledPath != "" {
        var configPath string
        if systemWide {
            configPath = "/etc/profile"
        } else if userName != "" {
            configPath = fmt.Sprintf("/home/%s/.profile", userName)
        } else {
            fmt.Println("Invalid installation option.")
            return
        }

        // Append to the chosen shell config file
        appendToShellConfig(configPath, packageInfo.InstalledPath)
        fmt.Printf("Successfully added %s to PATH.\n", packageInfo.InstalledPath)
    } else {
        fmt.Println("Installation successful, but 'installed_path' is not specified in package.json.")
    }
}

// copyFile copies the source file to the destination file.
func copyFile(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}
func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}
func listUsersWithHome() ([]string, error) {
    file, err := os.Open("/etc/passwd")
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var users []string
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        fields := strings.Split(scanner.Text(), ":")
        if len(fields) >= 6 && fields[5] != "" {
            users = append(users, fields[0])
        }
    }
    if err := scanner.Err(); err != nil {
        return nil, err
    }
    return users, nil
}

func appendToShellConfig(configPath, pathToAppend string) {
    // Check if the file exists
    if _, err := os.Stat(configPath); os.IsNotExist(err) {
        fmt.Printf("Shell configuration file %s not found.\n", configPath)
        return
    }

    // Read the file content
    content, err := ioutil.ReadFile(configPath)
    if err != nil {
        fmt.Printf("Error reading %s: %v\n", configPath, err)
        return
    }

    // Check if the path is already in the file
    if strings.Contains(string(content), pathToAppend) {
        fmt.Println("Already done.")
        return
    }

    // Append the path to the file
    f, err := os.OpenFile(configPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        fmt.Printf("Error opening %s: %v\n", configPath, err)
        return
    }
    defer f.Close()

    // Append the path
    _, err = f.WriteString(fmt.Sprintf("export PATH=$PATH:%s\n", pathToAppend))
    if err != nil {
        fmt.Printf("Error writing to %s: %v\n", configPath, err)
        return
    }
}


func uninstallPackage(packageName string, systemWide bool, userName string) {
	packageDir := filepath.Join("/packages/repos", packageName)
	if _, err := os.Stat(packageDir); os.IsNotExist(err) {
		fmt.Printf("Package %s not found.\n", packageName)
		return
	}

	packageInfoBytes, err := ioutil.ReadFile(filepath.Join(packageDir, "package.json"))
	if err != nil {
		fmt.Println("Error reading package.json:", err)
		return
	}

	var packageInfo PackageInfo
	if err := json.Unmarshal(packageInfoBytes, &packageInfo); err != nil {
		fmt.Println("Error parsing package.json:", err)
		return
	}

	if packageInfo.InstalledPath != "" {
		// Determine the config path based on installation scope
		var configPath string
		if systemWide {
			configPath = "/etc/profile"
		} else if userName != "" {
			configPath = fmt.Sprintf("/home/%s/.profile", userName)
		} else {
			fmt.Println("Invalid installation option.")
			return
		}

		// Read the profile file
		fileContent, err := ioutil.ReadFile(configPath)
		if err != nil {
			fmt.Printf("Error reading %s: %v\n", configPath, err)
			return
		}

		// Prepare a new file content without the package's PATH addition
		var newContent []string
		scanner := bufio.NewScanner(strings.NewReader(string(fileContent)))
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.Contains(line, packageInfo.InstalledPath) {
				newContent = append(newContent, line)
			}
		}

		// Write the modified content back to the file
		if err := ioutil.WriteFile(configPath, []byte(strings.Join(newContent, "\n")), 0644); err != nil {
			fmt.Printf("Error writing to %s: %v\n", configPath, err)
			return
		}

		fmt.Printf("Removed %s from PATH.\n", packageInfo.InstalledPath)

		// Remove the package directory
		if err := os.RemoveAll(packageInfo.InstalledPath); err != nil {
			fmt.Printf("Error removing %s: %v\n", packageInfo.InstalledPath, err)
		} else {
			fmt.Printf("Successfully uninstalled %s.\n", packageName)
		}
	} else {
		fmt.Println("Uninstallation successful, but 'installed_path' is not specified in package.json.")
	}
}
func listPackages() {
    dirPath := "/packages/store"
    files, err := ioutil.ReadDir(dirPath)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Installed packages:")
    for _, file := range files {
        if file.IsDir() {
            fmt.Println(file.Name())
        }
    }
}
func cloneRepo(repoURL, baseDestination string) {
	// Extract the repository name from the URL
	// This assumes the URL ends with the repository name
	repoName := filepath.Base(repoURL)
	destination := filepath.Join(baseDestination, repoName)

	cmd := exec.Command("git", "clone", repoURL, destination)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Failed to clone repository: %v\n", err)
	} else {
		fmt.Println("Repository cloned successfully.")
		// Append the local file path to /packages/repos.list
		listFilePath := filepath.Join(baseDestination, "repos.list")
		if err := os.WriteFile(listFilePath, []byte(destination+"\n"), os.ModeAppend|0644); err != nil {
			fmt.Printf("Failed to append to repos.list: %v\n", err)
		}
	}
}


func syncRepo(repoPath string) {
	cmd := exec.Command("git", "-C", repoPath, "pull")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Failed to sync repository: %v\n", err)
	} else {
		fmt.Println("Repository synced successfully.")
	}
}
//generate a world file
// GenerateWorldFile generates a world file of installed packages.
func GenerateWorldFile() error {
	// Define the path to the world file.
	worldFilePath := "/packages/world"

	// Ensure the world file directory exists.
	if err := os.MkdirAll(filepath.Dir(worldFilePath), 0755); err != nil {
		return fmt.Errorf("failed to create world file directory: %w", err)
	}

	// Open the world file for writing.
	worldFile, err := os.Create(worldFilePath)
	if err != nil {
		return fmt.Errorf("failed to create world file: %w", err)
	}
	defer worldFile.Close()

	// Traverse the /packages/store/ directory.
	err = filepath.Walk("/packages/store/", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if the current item is a package.json file.
		if info.Name() == "package.json" {
			// Read and parse the package.json file.
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read package.json: %w", err)
			}

			var pkgInfo PackageInfo
			if err := json.Unmarshal(data, &pkgInfo); err != nil {
				return fmt.Errorf("failed to parse package.json: %w", err)
			}

			// Write the package information to the world file.
			_, err = worldFile.WriteString(fmt.Sprintf("%s %s %s\n", pkgInfo.Name, pkgInfo.Version, pkgInfo.InstalledPath))
			if err != nil {
				return fmt.Errorf("failed to write to world file: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to traverse /packages/store/: %w", err)
	}

	return nil
}
func main() {
	installFlag := flag.String("a", "", "Installs package(s)")
	systemWideFlag := flag.Bool("s", false, "Install system-wide")
	userFlag := flag.String("u", "", "Install for a specific user")
	listFlag := flag.Bool("l", false, "List installed packages")
	cloneFlag := flag.String("e", "", "URL of the repository to clone")
	syncFlag := flag.Bool("sync", false, "Sync the repository")
	reposListFlag := flag.String("repos", "", "Path to the list of repositories to sync")
	uninstallFlag := flag.String("r", "", "Uninstalls package(s)") // New uninstall flag
	flag.Parse()

	if *listFlag {
		listPackages()
	} else if *installFlag != "" {
		if *systemWideFlag {
			installPackage(*installFlag, true, "") // Correctly passing three arguments
		} else if *userFlag != "" {
			installPackage(*installFlag, false, *userFlag) // Correctly passing three arguments
		} else {
			fmt.Println("Invalid command. Use '-a <pkg> -s' for system-wide or '-a <pkg> -u <user>' for user-specific.")
		}
	} else if *cloneFlag != "" {
		cloneRepo(*cloneFlag, "/packages/repos/")
	} else if *syncFlag {
		syncRepo("/packages/repos")
	} else if *reposListFlag != "" {
		// Read the file containing the list of repository paths
		content, err := os.ReadFile(*reposListFlag)
		if err != nil {
			fmt.Printf("Failed to read repos.list: %v\n", err)
			return
		}

		// Split the content by newline to get a slice of repository paths
		repoPaths := strings.Split(string(content), "\n")

		// Iterate over the slice and sync each repository
		for _, repoPath := range repoPaths {
			if repoPath != "" { // Skip empty lines
				syncRepo(repoPath)
			}
		}
	} else if *uninstallFlag != "" {
		// Assuming systemWideFlag and userFlag are still relevant for uninstallation
		// You might need to adjust this logic based on how you want to handle uninstallation
		if *systemWideFlag {
			uninstallPackage(*uninstallFlag, true, "") // Uninstall system-wide
		} else if *userFlag != "" {
			uninstallPackage(*uninstallFlag, false, *userFlag) // Uninstall for a specific user
		} else {
			fmt.Println("Invalid command. Use '-r <pkg> -s' for system-wide or '-r <pkg> -u <user>' for user-specific.")
		}
	} else {
		fmt.Println("Invalid command. Use '-a <pkg>' to install, '-l' to list installed packages, or '-r <pkg>' to uninstall.")
	}

	if err := GenerateWorldFile(); err != nil {
		fmt.Printf("Error generating world file: %v\n", err)
	} else {
		fmt.Println("World file generated successfully.")
	}
}
