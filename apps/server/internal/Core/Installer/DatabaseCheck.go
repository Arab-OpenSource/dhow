package installer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

const databaseCheckTimeout = 15 * time.Second

type PostgresChecker struct{}

func (PostgresChecker) ServerMajor(ctx context.Context, url string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, databaseCheckTimeout)
	defer cancel()
	pool, err := config.OpenDatabase(ctx, url)
	if err != nil {
		return 0, err
	}
	defer pool.Close()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return 0, fmt.Errorf("connect to database: %w", err)
	}
	defer conn.Release()
	return ParseServerMajor(conn.Conn().PgConn().ParameterStatus("server_version"))
}

func ParseServerMajor(v string) (int, error) {
	head, _, _ := strings.Cut(strings.TrimSpace(v), " ")
	head, _, _ = strings.Cut(head, ".")
	n, err := strconv.Atoi(head)
	if err != nil {
		return 0, fmt.Errorf("unrecognised server_version %q", v)
	}
	return n, nil
}
