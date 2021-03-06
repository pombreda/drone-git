package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/drone/drone-plugin-go/plugin"
)

var netrcFile = `
machine %s
login %s
password %s
`

func main() {
	c := new(plugin.Clone)
	plugin.Param("clone", c)
	plugin.Parse()

	err := os.MkdirAll(c.Dir, 0777)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// generate the .netrc file
	if err := writeNetrc(c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// write the rsa private key if provided
	if err := writeKey(c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var cmds []*exec.Cmd
	if isPR(c) || isTag(c) {
		cmds = append(cmds, clone(c))
		cmds = append(cmds, fetch(c))
		cmds = append(cmds, checkoutHead(c))
	} else {
		cmds = append(cmds, cloneBranch(c))
		cmds = append(cmds, checkoutSha(c))
	}

	for _, cmd := range cmds {
		cmd.Dir = c.Dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		trace(cmd)
		err := cmd.Run()
		if err != nil {
			os.Exit(1)
		}
	}
}

// Returns true if cloning a pull request.
func isPR(in *plugin.Clone) bool {
	return strings.HasPrefix(in.Ref, "refs/pull/")
}

func isTag(in *plugin.Clone) bool {
	return strings.HasPrefix(in.Ref, "refs/tags/")
}

// Clone executes a git clone command.
func clone(c *plugin.Clone) *exec.Cmd {
	return exec.Command(
		"git",
		"clone",
		"--depth=50",
		"--recursive",
		c.Remote,
		c.Dir,
	)
}

// CloneBranch executes a git clone command
// for a single branch.
func cloneBranch(c *plugin.Clone) *exec.Cmd {
	return exec.Command(
		"git",
		"clone",
		"-b",
		c.Branch,
		"--depth=50",
		"--recursive",
		c.Remote,
		c.Dir,
	)
}

// Checkout executes a git checkout command.
func checkoutSha(c *plugin.Clone) *exec.Cmd {
	return exec.Command(
		"git",
		"checkout",
		"-qf",
		c.Sha,
	)
}

// Checkout executes a git checkout command.
func checkoutHead(c *plugin.Clone) *exec.Cmd {
	return exec.Command(
		"git",
		"checkout",
		"-qf",
		"FETCH_HEAD",
	)
}

// Fetch executes a git fetch to origin.
func fetch(c *plugin.Clone) *exec.Cmd {
	return exec.Command(
		"git",
		"fetch",
		"origin",
		fmt.Sprintf("+%s:", c.Ref),
	)
}

// Trace writes each command to standard error (preceded by a ‘$ ’) before it
// is executed. Used for debugging your build.
func trace(cmd *exec.Cmd) {
	fmt.Println("$", strings.Join(cmd.Args, " "))
}

// Writes the netrc file.
func writeNetrc(in *plugin.Clone) error {
	if len(in.Netrc.Machine) == 0 {
		return nil
	}
	out := fmt.Sprintf(
		netrcFile,
		in.Netrc.Machine,
		in.Netrc.Login,
		in.Netrc.Password,
	)
	u, err := user.Current()
	if err != nil {
		return err
	}
	path := filepath.Join(u.HomeDir, ".netrc")
	return ioutil.WriteFile(path, []byte(out), 0600)
}

// Writes the RSA private key
func writeKey(in *plugin.Clone) error {
	if len(in.Keypair.Private) == 0 {
		return nil
	}
	u, err := user.Current()
	if err != nil {
		return err
	}
	sshpath := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshpath, 0700); err != nil {
		return err
	}
	confpath := filepath.Join(sshpath, "config")
	privpath := filepath.Join(sshpath, "id_rsa")
	ioutil.WriteFile(confpath, []byte("StrictHostKeyChecking no\n"), 0700)
	return ioutil.WriteFile(privpath, []byte(in.Keypair.Private), 0600)
}
