package chroot

import (
	"os"
	"syscall"
)

// Enter enters chroot and returns function to exit
func Enter(dir string) (exitFn func() error, err error) {
	var root *os.File
	var curr *os.File

	exit := func() (err error) {
		defer func() {
			if root != nil {
				_ = root.Close()
			}
			if curr != nil {
				if err2 := curr.Chdir(); err == nil {
					err = err2
				}
				_ = curr.Close()
			}
		}()

		if root != nil {
			if err := root.Chdir(); err != nil {
				return err
			}
			if err := syscall.Chroot("."); err != nil {
				return err
			}
		}
		return nil
	}
	defer func() {
		if err != nil {
			_ = exit()
		}
	}()

	root, err = os.Open("/")
	if err != nil {
		return nil, err
	}

	curr, err = os.Open(".")
	if err != nil {
		return nil, err
	}

	if err := syscall.Chroot(dir); err != nil {
		return nil, err
	}
	if err := os.Chdir("/"); err != nil {
		return nil, err
	}

	return exit, nil
}
