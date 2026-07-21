package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dsn := flag.String("dsn", os.Getenv("SPIKE2_DSN"), "postgres DSN")
	seed := flag.Bool("seed", false, "insert one admitted submission and print its id, then exit")
	crashSubmission := flag.Int("crash-submission", 0, "submission id to crash on (0 = no crash injected)")
	crashStage := flag.String("crash-stage", "", "stage name to crash at: validate|normalize|evaluate|alert|evidence")
	crashTiming := flag.String("crash-timing", "", "before|mid|after")
	flag.Parse()

	if *dsn == "" {
		log.Fatal("dsn required (set -dsn or SPIKE2_DSN)")
	}

	db, err := sql.Open("pgx", *dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	if *seed {
		var id int
		if err := db.QueryRowContext(ctx,
			`INSERT INTO submissions DEFAULT VALUES RETURNING id`).Scan(&id); err != nil {
			log.Fatalf("seed submission: %v", err)
		}
		fmt.Println(id)
		return
	}

	crash := CrashSpec{SubmissionID: *crashSubmission, Stage: *crashStage, Timing: *crashTiming}
	if err := runWorkerLoop(ctx, db, crash); err != nil {
		log.Fatalf("worker loop: %v", err)
	}
}
