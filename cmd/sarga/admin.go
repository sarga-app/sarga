package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	domauth "github.com/sarga-app/sarga/internal/domain/auth"
	"github.com/sarga-app/sarga/internal/support/config"
	"github.com/sarga-app/sarga/internal/support/db"
	"github.com/sarga-app/sarga/internal/support/db/queries"
)

var adminConfigPath string

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Administrative operations",
}

var createUserCmd = &cobra.Command{
	Use:   "create-user",
	Short: "Create a local user account",
	RunE:  runCreateUser,
}

var createUserFlags struct {
	username string
	email    string
	password string
	admin    bool
}

func runCreateUser(cmd *cobra.Command, _ []string) error {
	if createUserFlags.username == "" {
		return errors.New("--username is required")
	}

	pw, err := resolvePassword(createUserFlags.password)
	if err != nil {
		return err
	}

	hash, err := domauth.Hash(pw)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	cfg, err := config.Load(adminConfigPath)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	dbPath := dbPathFromConfig(cfg)
	if err = db.MigrateUp(ctx, dbPath); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	dbInst, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer dbInst.Close() //nolint:errcheck

	q := queries.New(dbInst.Write())

	now := time.Now().UTC().Format(time.RFC3339)
	var emailPtr *string
	if createUserFlags.email != "" {
		e := strings.ToLower(strings.TrimSpace(createUserFlags.email))
		emailPtr = &e
	}

	user, err := q.CreateUser(ctx, queries.CreateUserParams{
		ID:                uuid.NewString(),
		Username:          strings.ToLower(strings.TrimSpace(createUserFlags.username)),
		Email:             emailPtr,
		PasswordHash:      &hash,
		IsAdmin:           createUserFlags.admin,
		PasswordChangedAt: &now,
	})
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	fmt.Println(user.ID)
	return nil
}

func resolvePassword(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		pw, err := readLine(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read password from stdin: %w", err)
		}
		return pw, nil
	}
	fmt.Fprint(os.Stderr, "Password: ")
	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	if len(raw) == 0 {
		return "", errors.New("password must not be empty")
	}
	return string(raw), nil
}

func readLine(f *os.File) (string, error) {
	buf := make([]byte, 0, 128)
	one := make([]byte, 1)
	for {
		n, err := f.Read(one)
		if n > 0 && one[0] == '\n' {
			break
		}
		if n > 0 {
			buf = append(buf, one[0])
		}
		if err != nil {
			break
		}
	}
	s := strings.TrimRight(string(buf), "\r")
	if s == "" {
		return "", errors.New("password must not be empty")
	}
	return s, nil
}

func init() {
	adminCmd.PersistentFlags().StringVarP(&adminConfigPath, "config", "c", "config.yaml",
		"path to config file (optional; defaults applied if missing)")

	createUserCmd.Flags().StringVar(&createUserFlags.username, "username", "", "username (required, lowercased)")
	createUserCmd.Flags().StringVar(&createUserFlags.email, "email", "", "email address (optional)")
	createUserCmd.Flags().StringVar(&createUserFlags.password, "password", "", "password (omit to be prompted securely)")
	createUserCmd.Flags().BoolVar(&createUserFlags.admin, "admin", false, "grant admin role")

	adminCmd.AddCommand(createUserCmd)
	rootCmd.AddCommand(adminCmd)
}
