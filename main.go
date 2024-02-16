package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

/*
 * This is a CLI application that can track file changes, similar to Git.
 * Usually you want to divide the logic into different files, but HyperSkill doesn't allow it for
 * some reason (the tests fail). So I will leave it as it is. HyperSkill has forced my hand.
 */

// Command struct holds the name, description, and handler function of each command.
type Command struct {
	Name        string              // Name of the command
	Description string              // Description of the command
	Handler     func(args []string) // Handler function for the command
}

type Commit struct {
	HashID  string
	Author  string
	Message string
}

const (
	configPath    = "vcs/config.txt"
	indexFilePath = "vcs/index.txt"
	commitDir     = "vcs/commits"
	logFilePath   = "vcs/log.txt"
)

var (
	// Commands holds the list of commands in order.
	Commands = []Command{
		{Name: "config", Description: "Get and set a username.", Handler: handleConfig},
		{Name: "add", Description: "Add a file to the index.", Handler: handleAdd},
		{Name: "log", Description: "Show commit logs.", Handler: handleLog},
		{Name: "commit", Description: "Save changes.", Handler: handleCommit},
		{Name: "checkout", Description: "Restore a file.", Handler: handleCheckout},
	}
)

func main() {
	// Ensure the vcs directory exists
	err := os.MkdirAll("./vcs", os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	setupCommands()
}

func setupCommands() {
	// If no command provided or help flag is used, print help message
	if len(os.Args) < 2 || os.Args[1] == "--help" {
		printHelp()
		return
	}

	commandName := os.Args[1]

	// Find and execute the appropriate command handler
	for _, cmd := range Commands {
		if cmd.Name == commandName {
			cmd.Handler(os.Args[2:])
			return
		}
	}

	// Print error if the command is not recognized
	fmt.Printf("'%s' is not a SVCS command.\n", commandName)
}

func printHelp() {
	// Print list of available commands and their descriptions
	fmt.Println("These are SVCS commands:")
	for _, cmd := range Commands {
		fmt.Printf("%-30s %s\n", cmd.Name, cmd.Description)
	}
}

func handleConfig(args []string) {
	content, _ := os.ReadFile(configPath)
	if len(args) > 0 {
		setupConfig(args[0])
	} else if len(content) != 0 {
		fmt.Printf("The username is %s.", content)
	} else {
		fmt.Println("Please, tell me who you are.")
	}
}

func handleAdd(args []string) {
	// Check if the index file exists
	if _, err := os.Stat(indexFilePath); os.IsNotExist(err) {
		// If the index file does not exist, create it
		file, err := os.Create(indexFilePath)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
	}

	// Read the content of the index file
	content, err := os.ReadFile(indexFilePath)
	if err != nil {
		log.Fatal(err)
	}

	if len(args) > 0 {
		setupAdd(args[0])
	} else if len(content) != 0 {
		fmt.Println("Tracked files:")
		fmt.Println(string(content))
	} else {
		fmt.Println("Add a file to the index.")
	}
}

func handleLog(args []string) {
	if len(args) > 0 {
		fmt.Println("Too many arguments.")
		return
	}
	readCommits()
}

func handleCommit(args []string) {
	// Combine all arguments into a single commit message
	message := getMessageFromArgs(args)

	// Check if a message was provided
	if message == "" {
		fmt.Println("Message was not passed.")
		return
	}
	// Check if there are files in the index
	if isIndexEmpty() {
		fmt.Println("Nothing to commit.")
		return
	}

	// Check for changes compared to the last commit
	changes := compareWithLastCommit()

	// If there are no changes compared to the last commit, print a message
	if !changes {
		fmt.Println("Nothing to commit.")
		return
	}

	// Create a new commit
	newCommit := createCommit(message)

	// Generate a commit ID
	commitID, err := newCommit.createId()
	if err != nil {
		log.Fatal(err)
	}
	newCommit.HashID = commitID

	// Create the commit directory
	commitDirPath, err := newCommit.createCommitDir()
	if err != nil {
		log.Fatal(err)
	}

	// Copy files to the new commit directory
	copyFilesToCommitDir(commitDirPath)

	// Create a log entry for the new commit
	newCommit.createLog()

	fmt.Println("Changes are committed.")
}

/*
The checkout command must be passed to the program together with the commit ID to indicate which
commit should be used. If a commit with the given ID exists, the contents of the tracked file
should be restored in accordance with this commit.
*/
func handleCheckout(args []string) {
	if len(args) != 1 {
		fmt.Println("Commit id was not passed.")
		return
	}

	switchCommit(args[0])
}

/*
CONFIG
*/
func doesConfigExist() bool {
	// Check if config file exists
	if _, err := os.Stat(configPath); err == nil {
		return true
	}
	return false
}

func readConfig() string {
	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatal(err)
	}
	return string(data)
}

func setupConfig(name string) {
	// Check if config file exists
	if doesConfigExist() && name == "" {
		fmt.Printf("The username is %s.\n", readConfig())
		return
	} else if name == "" {
		fmt.Println("Please, tell me who you are.")
		return
	}

	// Write new username to config file
	err := os.WriteFile(configPath, []byte(name), 0644)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("The username is %s.\n", name)
}

/*
	ADD
*/

func setupAdd(file string) {
	// Check if no file is provided and the index is not empty
	if file == "" && !isIndexEmpty() {
		readIndex()
		return
	} else if file == "" && isIndexEmpty() {
		fmt.Println("Add a file to the index.")
		return
	}

	// Check if the file exists
	if _, err := os.Stat(file); os.IsNotExist(err) {
		fmt.Printf("Can't find '%s'.\n", file)
		return
	}

	// Check if the file is already tracked in the index
	if isFileTracked(file) {
		// Print a message indicating that the file is already tracked
		fmt.Printf("The file '%s' is already tracked.\n", file)
		return
	}

	// Append file to index
	err := createIndex(file)
	if err != nil {
		log.Println("Error tracking file:", err)
		return
	}
	// Print a message indicating that the file has been successfully tracked
	fmt.Printf("The file '%s' is tracked.\n", file)
}

func isFileTracked(filePath string) bool {
	// Read the content of the index file
	indexContent, err := os.ReadFile(indexFilePath)
	if err != nil {
		log.Fatal(err)
	}

	// Split the content of the index file into lines
	filePaths := strings.Split(string(indexContent), "\n")

	// Check if the file path exists in the index
	for _, path := range filePaths {
		if path == filePath {
			return true
		}
	}
	return false
}

func createIndex(addedFile string) error {
	// Open index file in append mode or create it if not exist
	file, err := os.OpenFile(indexFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Append new file name followed by a newline character
	_, err = file.WriteString(addedFile + "\n")
	if err != nil {
		return err
	}
	return nil
}

func readIndex() {
	// Read index file
	data, err := os.ReadFile(indexFilePath)
	if err != nil {
		fmt.Println("No commits yet.")
		return
	}
	fmt.Printf("Tracked files:\n%s", string(data))
}

func isIndexEmpty() bool {
	// Get index file info
	info, err := os.Stat(indexFilePath)
	if err != nil {
		return true
	}
	// Check if index file is empty
	return info.Size() == 0
}

/*
COMMITS
*/

func (c Commit) createId() (string, error) {
	// Read the content of the index file
	indexContent, err := os.ReadFile(indexFilePath)
	if err != nil {
		return "", err
	}

	// Check if the index content is empty
	if len(indexContent) == 0 {
		return "", errors.New("nothing to commit")
	}

	// Get the current timestamp
	timestamp := time.Now().UnixNano()

	// Append the timestamp to the index content
	contentWithTimestamp := append(indexContent, []byte(fmt.Sprintf("%d", timestamp))...)

	// Calculate the hash of the index content with the timestamp to generate the ID
	id := hashContent(contentWithTimestamp)

	return id, nil
}

func hashContent(content []byte) string {
	// Create a new SHA-256 hash
	hash := sha256.New()

	// Write the content of the files into the hash
	_, err := hash.Write(content)
	if err != nil {
		log.Fatal(err)
	}

	// Calculate the hash and return it as a hexadecimal string
	hashInBytes := hash.Sum(nil)
	return fmt.Sprintf("%x", hashInBytes)
}

func (c Commit) createCommitDir() (string, error) {
	var commitDirPath = commitDir + "/" + c.HashID
	// Check if the vcs/commits/id directory exists; if not, create it
	if _, err := os.Stat(commitDirPath); os.IsNotExist(err) {
		// Create a new directory for the commit
		commitDirPath = fmt.Sprintf("%s/%s", commitDir, c.HashID)
		err := os.Mkdir(commitDirPath, os.ModePerm)
		if err != nil {
			return "", err
		}

	} else {
		commitDirPath = fmt.Sprintf("%s/%s", commitDir, c.HashID)
	}
	return commitDirPath, nil
}

func getMessageFromArgs(args []string) string {
	return strings.TrimSpace(strings.Join(args, " "))
}

func createCommit(message string) Commit {
	// Open the index file to read the list of files
	indexFile, err := os.Open(indexFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer indexFile.Close()

	return Commit{
		Author:  readConfig(),
		Message: message,
	}
}

func compareWithLastCommit() bool {
	// Retrieve the hash ID of the last commit
	lastCommitID := getLastCommitID()

	if lastCommitID == "" {
		return true
	}

	// Read the list of file paths from the index file
	indexContent, err := os.ReadFile(indexFilePath)
	if err != nil {
		log.Fatal(err)
	}

	// Split the content of the index file into lines
	filePaths := strings.Split(string(indexContent), "\n")

	// Check if there are changes compared to the last commit
	return hasChanges(filePaths, lastCommitID)
}

func getLastCommitID() string {
	// Check if the vcs/commits directory exists; if not, create it
	if _, err := os.Stat(commitDir); os.IsNotExist(err) {
		err := os.MkdirAll(commitDir, os.ModePerm)
		if err != nil {
			return ""
		}
	}

	// Read the list of entries in the commits in log.txt
	logContent, err := os.ReadFile(logFilePath)
	if err != nil {
		return ""
	}
	// Get the commit ID from the first line of the log file
	commitID := strings.Split(string(logContent), "\n")[0]
	return strings.TrimPrefix(commitID, "commit ")
}

func hasChanges(filePaths []string, commitDirPath string) bool {
	// Iterate over all files in the commit directory
	for _, filePath := range filePaths {
		// If the file name is empty, continue with the next one
		if filePath == "" {
			continue
		}

		// Check if there are changes for the current file
		if fileHasChanges(filePath, commitDirPath) {
			return true
		}
	}

	return false
}

func fileHasChanges(filePath, commitDirPath string) bool {
	// Get the path relative to the commit directory
	relativePath := strings.TrimPrefix(filePath, commitDir)

	// Check if the file exists in the last commit
	lastCommitFile := filepath.Join(commitDir, commitDirPath, relativePath)
	if _, err := os.Stat(lastCommitFile); err == nil {
		// If the file exists, read its content and calculate its hash
		lastCommitFileContent, err := os.ReadFile(lastCommitFile)
		if err != nil {
			log.Fatal(err)
		}
		lastCommitFileHash := hashContent(lastCommitFileContent)

		// Read the content of the current file
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatal(err)
		}

		// Calculate the hash of the current file
		currentFileHash := hashContent(fileContent)

		// Compare hashes
		return lastCommitFileHash != currentFileHash
	}

	return true // If the file doesn't exist in the last commit, there are changes
}

func copyFilesToCommitDir(commitDirPath string) {
	// Check if the vcs/commits directory exists; if not, create it
	if _, err := os.Stat(commitDir); os.IsNotExist(err) {
		err := os.MkdirAll(commitDir, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Read the list of file paths from the index file
	indexContent, err := os.ReadFile(indexFilePath)
	if err != nil {
		log.Fatal(err)
	}

	// Split the content of the index file into lines
	filePaths := strings.Split(string(indexContent), "\n")

	// Copy each file listed in the index into the new commit directory
	for _, filePath := range filePaths {
		// If the file name is empty, continue with the next one
		if filePath == "" {
			continue
		}

		// Construct the destination file path using filepath.Join
		destination := filepath.Join(commitDirPath, strings.TrimPrefix(filePath, "vcs/"))

		// Copy the file into the commit directory
		err := copyFile(filePath, destination)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func copyFile(src, dst string) error {
	// Abrir el archivo origen para lectura
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Crear el archivo destino
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copiar el contenido del archivo origen al archivo destino
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	// Flush para asegurar que todos los datos se escriban en disco
	err = dstFile.Sync()
	if err != nil {
		return err
	}

	return nil
}

func findCommitById(id string) *Commit {
	// Check if the commit directory exists
	commitDirPath := filepath.Join(commitDir, id)
	if _, err := os.Stat(commitDirPath); os.IsNotExist(err) {
		return nil
	}

	// Read the commit message from the commit log file
	logContent, err := os.ReadFile(logFilePath)
	if err != nil {
		log.Fatal(err)
	}

	// Split the log content into individual commit entries
	commitEntries := strings.Split(string(logContent), "\n\n")

	// Iterate over each commit entry
	for _, entry := range commitEntries {
		// Split the entry into lines
		lines := strings.Split(entry, "\n")

		// Extract the commit ID from the first line
		commitID := strings.TrimPrefix(lines[0], "commit ")

		// Check if the commit ID matches the provided ID
		if commitID == id {
			// Extract author and message from the commit entry
			var author, message string
			for _, line := range lines {
				if strings.HasPrefix(line, "Author: ") {
					author = strings.TrimPrefix(line, "Author: ")
				} else if !strings.HasPrefix(line, "commit ") {
					message += line + "\n"
				}
			}

			// Return a pointer to the Commit struct
			return &Commit{
				HashID:  commitID,
				Author:  author,
				Message: strings.TrimSpace(message),
			}
		}
	}

	// If the commit ID is not found, return nil
	return nil
}

/*
LOG
*/

func (c Commit) createLog() {
	// Prepare the new commit information
	newCommitInfo := fmt.Sprintf("commit %s\nAuthor: %s\n%s\n\n", c.HashID, c.Author, c.Message)

	// Read the existing log content
	existingLogContent, err := os.ReadFile(logFilePath)
	if err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}

	// Append the new commit information to the existing log content
	updatedLogContent := append([]byte(newCommitInfo), existingLogContent...)

	// Write the updated log content back to the log file
	err = os.WriteFile(logFilePath, updatedLogContent, 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func readCommits() {
	// Read the list of entries in the commits directory
	entries, err := os.ReadDir(commitDir)
	if err != nil {
		fmt.Println("No commits yet.")
		return
	}

	// Check if there are any commit directories
	if len(entries) == 0 {
		fmt.Println("No commits yet.")
		return
	}

	// Read from the log.txt
	logContent, err := os.ReadFile(logFilePath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(logContent))
}

/*
CHECKOUT
*/
func switchCommit(commitID string) {
	// Check if the commit exists
	commit := findCommitById(commitID)
	if commit == nil {
		fmt.Println("Commit does not exist.")
		return
	}

	// Get the list of files in the commit directory
	commitDirPath := filepath.Join(commitDir, commitID)
	commitFiles, err := os.ReadDir(commitDirPath)
	if err != nil {
		log.Fatal(err)
	}

	// Get the list of files in the current directory
	//currentFiles, err := os.ReadDir(".")
	//if err != nil {
	//	log.Fatal(err)
	//}

	// Create a map to store the names of files in the commit
	commitFileMap := make(map[string]struct{})
	for _, file := range commitFiles {
		commitFileMap[file.Name()] = struct{}{}
	}

	// Copy files from the commit to the current directory
	for _, file := range commitFiles {
		source := filepath.Join(commitDirPath, file.Name())
		destination := filepath.Join(".", file.Name())

		err := copyFile(source, destination)
		if err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("Switched to commit %s.\n", commitID)
}
