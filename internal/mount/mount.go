// Package mount handles filesystem mounting for the immutable root.
// Sets up squashfs, overlayfs, and tmpfs as needed during boot.
package mount

import (
	"fmt"
	"log"
	"os"
	"syscall"
)

// Config holds the mount configuration for the immutable root.
type Config struct {
	// SquashfsImage is the path to the squashfs root image.
	SquashfsImage string
	// MountPoint is where the final root gets mounted.
	MountPoint string
	// UpperDir is the tmpfs-backed writable overlay.
	UpperDir string
	// WorkDir is the overlayfs work directory.
	WorkDir string
	// CacheSize is the tmpfs size limit (e.g., "1G").
	CacheSize string
}

// DefaultConfig returns the default mount configuration.
func DefaultConfig() *Config {
	return &Config{
		SquashfsImage: "/dev/sda1",
		MountPoint:    "/",
		UpperDir:      "/tmp/upper",
		WorkDir:       "/tmp/work",
		CacheSize:     "1G",
	}
}

// SetupImmutableRoot mounts the squashfs root with overlayfs.
// Only call this when running as PID 1.
func SetupImmutableRoot(cfg *Config) error {
	log.Printf("mount: setting up immutable root")

	// Create mount points
	for _, dir := range []string{"/ro-root", cfg.UpperDir, cfg.WorkDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mount: create %s: %w", dir, err)
		}
	}

	// Mount squashfs read-only
	if err := syscall.Mount(cfg.SquashfsImage, "/ro-root", "squashfs", syscall.MS_RDONLY, ""); err != nil {
		return fmt.Errorf("mount: squashfs: %w", err)
	}
	log.Printf("mount: squashfs mounted RO at /ro-root")

	// Mount tmpfs for upper and work dirs
	tmpfsOpts := fmt.Sprintf("size=%s,mode=0755", cfg.CacheSize)
	if err := syscall.Mount("tmpfs", "/tmp", "tmpfs", 0, tmpfsOpts); err != nil {
		return fmt.Errorf("mount: tmpfs: %w", err)
	}
	log.Printf("mount: tmpfs mounted at /tmp (%s)", cfg.CacheSize)

	// Re-create upper/work after tmpfs mount
	for _, dir := range []string{cfg.UpperDir, cfg.WorkDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mount: create %s: %w", dir, err)
		}
	}

	// Mount overlayfs
	overlayOpts := fmt.Sprintf("lowerdir=/ro-root,upperdir=%s,workdir=%s", cfg.UpperDir, cfg.WorkDir)
	if err := syscall.Mount("overlay", cfg.MountPoint, "overlay", 0, overlayOpts); err != nil {
		return fmt.Errorf("mount: overlayfs: %w", err)
	}
	log.Printf("mount: overlayfs active (lower=/ro-root, upper=%s)", cfg.UpperDir)

	return nil
}

// SetupDevMode creates local directories for development without real mounts.
func SetupDevMode(cacheDir, queueDir string) error {
	log.Printf("mount: dev mode — skipping real mounts")
	for _, dir := range []string{cacheDir, queueDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("mount: create dev dir %s: %w", dir, err)
		}
	}
	return nil
}

// MountProc mounts /proc (needed when running as PID 1).
func MountProc() error {
	if err := os.MkdirAll("/proc", 0555); err != nil {
		return fmt.Errorf("mount: create /proc: %w", err)
	}
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("mount: /proc: %w", err)
	}
	return nil
}

// MountSys mounts /sys (needed when running as PID 1).
func MountSys() error {
	if err := os.MkdirAll("/sys", 0555); err != nil {
		return fmt.Errorf("mount: create /sys: %w", err)
	}
	if err := syscall.Mount("sysfs", "/sys", "sysfs", 0, ""); err != nil {
		return fmt.Errorf("mount: /sys: %w", err)
	}
	return nil
}
