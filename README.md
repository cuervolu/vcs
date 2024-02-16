# Version Control System (Go)

This is a CLI application that can track file changes, similar to Git.
Usually you want to divide the logic into different files, but HyperSkill doesn't allow it for some reason (the tests fail). So I will leave it as it is. HyperSkill has forced my hand.

This is a simple version control system that can track file changes, similar to Git. It can track changes in files and restore the state of the project.

The program has the following commands:
- `config` - sets the username. The program uses the user name to save the commit information.
- `add` - adds a file to the staging area
- `commit` - saves the changes to the file
- `log` - shows the history of commits
- `checkout` - restores the file to a specific commit

The program uses a simple file system to store the files and their changes. The program stores the files in the `.vcs` directory in the root of the project. The program creates a new directory for each commit with unique ID and stores the files in it.  The program stores the commit information in the `log.txt` file. It stores the commit ID, the username, and the commit message.

In the `config` command, the program saves the username in the `config.txt` file. The program uses the username to save the commit information.

In the `index.txt` file, the program stores the files in the staging area. The program uses the `add` command to add the file to the staging area. The program uses the `commit` command to save the changes to the file.