module github.com/example/dayz-standalone-mode-updater

go 1.22

require (
	github.com/multiplay/go-battleye v0.0.0-20170307214542-2f9a4e4e5f6d
	github.com/pkg/sftp v1.13.6
	github.com/spf13/cobra v1.8.1
	golang.org/x/crypto v0.31.0
)


replace github.com/spf13/cobra => ./third_party/cobra

replace github.com/multiplay/go-battleye => ./third_party/go-battleye
replace github.com/pkg/sftp => ./third_party/sftp
replace golang.org/x/crypto => ./third_party/crypto
