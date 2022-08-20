# Opswat Engineering Assessment

This repo is my submission for the Opswat Engineering Assessment. I chose to use Go for this project due to its simple yet strongly-typed, web-native features.

To install, please run my install script **with sudo permissions**, `install.sh`:
```bash
sudo install.sh
```

Then, in `main.go` modify the `apiKey` constant on line 19 with your API key.
```golang
const apiKey = "YOURAPIKEY"
```

Run the `main.go` file against a file of your choice:
```golang
go run main.go "Engineering Assessment.pdf"
```