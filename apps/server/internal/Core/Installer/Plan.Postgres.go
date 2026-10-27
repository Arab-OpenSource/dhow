package installer

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	postgresWait     = 60 * time.Second
	postgresUser     = "postgres"
	databaseExistsQ  = "SELECT 1 FROM pg_database WHERE datname = 'dhow'"
	roleAndDatabaseQ = `SELECT 'CREATE ROLE dhow LOGIN' WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'dhow')\gexec
ALTER ROLE dhow WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD :'verifier';
SELECT 'CREATE DATABASE dhow OWNER dhow' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'dhow')\gexec
`
	dropDatabaseQ = `DROP DATABASE IF EXISTS dhow WITH (FORCE);
DROP ROLE IF EXISTS dhow;
`
)

func postgresBin(d Distro, name string) string {
	if d.Family == FamilyDnf {
		return "/usr/pgsql-18/bin/" + name
	}
	return "/usr/lib/postgresql/18/bin/" + name
}

func PostgresDataDir(d Distro) string {
	if d.Family == FamilyDnf {
		return "/var/lib/pgsql/18/data"
	}
	return "/var/lib/postgresql/18/main"
}

func asPostgres(d Distro, sql string, args ...string) Command {
	argv := append([]string{"-u", postgresUser, "--", postgresBin(d, "psql"), "-X", "-q", "-v", "ON_ERROR_STOP=1", "-d", "postgres"}, args...)
	c := Command{Name: "runuser", Args: argv, Dir: "/"}
	if sql != "" {
		c.Args = append(c.Args, "-f", "-")
		c.Stdin = []byte(sql)
	}
	return c
}

func (b *Builder) postgresSteps() []Step {
	d := b.Det.Distro
	var steps []Step
	if d.Family == FamilyDnf {
		dataDir := PostgresDataDir(d)
		initdb := Cmd(postgresBin(d, "postgresql-18-setup"), "initdb")
		steps = append(steps, Step{
			Title:    b.M.T(KeyStepInitdb),
			Files:    []string{dataDir + "/"},
			Commands: []string{initdb.String()},
			Done:     func(context.Context) (bool, error) { return exists(b.Sys, dataDir+"/PG_VERSION"), nil },
			Apply: func(ctx context.Context) error {
				_, err := run(ctx, b.Sys, initdb)
				return err
			},
		})
	}
	svc := d.PostgresService()
	check := Cmd("systemctl", "is-active", "--quiet", svc)
	steps = append(steps, CommandStep(b.Sys, b.M.T(KeyStepPostgresService, svc), &check,
		[]Command{Cmd("systemctl", "enable", "--now", svc)}, nil))
	return steps
}

func (b *Builder) waitPostgres(ctx context.Context) error {
	deadline := b.Sys.Now().Add(postgresWait)
	for {
		_, err := run(ctx, b.Sys, asPostgres(b.Det.Distro, "", "-c", "SELECT 1"))
		if err == nil {
			return nil
		}
		if !b.Sys.Now().Before(deadline) {
			return err
		}
		b.sleep(time.Second)
	}
}

func (b *Builder) databaseExists(ctx context.Context) bool {
	out, err := b.Sys.Run(ctx, asPostgres(b.Det.Distro, "", "-tA", "-c", databaseExistsQ))
	return err == nil && strings.TrimSpace(string(out.Stdout)) == "1"
}

func LocalDatabaseURL(password string) string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(DatabaseRole, password),
		Path:     "/" + DatabaseName,
		RawQuery: "host=" + PostgresSocket + "&port=" + strconv.Itoa(postgresPort) + "&sslmode=require",
	}
	return u.String()
}

func passwordFromURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.User == nil {
		return ""
	}
	pw, _ := u.User.Password()
	return pw
}

func (b *Builder) databaseStep() Step {
	wroteURL := false
	return Step{
		Title: b.M.T(KeyStepDatabase),
		Files: []string{DatabaseURLPath},
		Done: func(ctx context.Context) (bool, error) {
			return exists(b.Sys, DatabaseURLPath) && b.databaseExists(ctx), nil
		},
		Apply: func(ctx context.Context) error {
			if err := b.waitPostgres(ctx); err != nil {
				return err
			}
			password := passwordFromURL(string(b.readExisting(DatabaseURLPath)))
			if password == "" {
				pw, err := RandomHex(DBPasswordBytes)
				if err != nil {
					return err
				}
				password = pw
			}
			verifier, err := ScramVerifier(password)
			if err != nil {
				return err
			}
			script := "\\set verifier '" + verifier + "'\n" + roleAndDatabaseQ
			if _, err := run(ctx, b.Sys, asPostgres(b.Det.Distro, script)); err != nil {
				return err
			}
			if exists(b.Sys, DatabaseURLPath) {
				return nil
			}
			wroteURL = true
			return b.Sys.WriteFile(DatabaseURLPath, []byte(LocalDatabaseURL(password)+"\n"), modeRootPrivate)
		},
		Undo: func(context.Context) error {
			if wroteURL {
				return b.Sys.Remove(DatabaseURLPath)
			}
			return nil
		},
	}
}

func dropLocalDatabase(ctx context.Context, sys System, d Distro) error {
	if !exists(sys, postgresBin(d, "psql")) {
		return errors.New("psql for PostgreSQL 18 is not installed")
	}
	_, err := run(ctx, sys, asPostgres(d, dropDatabaseQ))
	return err
}
