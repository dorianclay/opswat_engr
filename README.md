# Opswat Engineering Assessment

This repo is my submission for the Opswat Engineering Assessment. I chose to use Go for this project due to its simple yet strongly-typed, web-native features.

To install, please run my install script, `install.sh`, in the current environment:

```bash
. install.sh
```

Then, in `main.go`, modify the `apiKey` constant on line 19 with your API key.

```golang
const apiKey = "YOURAPIKEY"
```

Run the `main.go` file against a file of your choice:

```golang
go run main.go "Engineering Assessment.pdf"
```

*If the above command failed* due to a `Command go not found` error, the script likedly did not execute in the shell's working environment. In this case add Go to the PATH, then run the command again:

```bash
export PATH=$PATH:/usr/local/go/bin
go run main.go "Engineering Assessment.pdf"
```