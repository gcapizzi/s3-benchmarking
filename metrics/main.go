package main

import (
	"log"
	"os"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"fmt"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

func main() {
	db, err := sql.Open(
		"mysql",
		fmt.Sprintf("root:%s@tcp(%s:3306)/%s",
		os.Getenv("MYSQL_ROOT_PASSWORD"),
		os.Getenv("MYSQL_HOST"),
		os.Getenv("MYSQL_DATABASE")))
	if err != nil {
		log.Fatalf("Failed to connect to db: %s", err.Error())
	}

	rows, err := db.Query(
		fmt.Sprintf(
			"SELECT blobstore_size, cf_push_time FROM %s",
			os.Getenv("MYSQL_DATABASE_TABLE"),
		),
	)
	if err != nil {
		log.Fatalf("Fetching metrics from db failed: %s", err.Error())
	}

	var blobstoreSizes []float64
	var cfPushTimes []float64

	for rows.Next() {
		var blobstoreSize int
		var cfPushTime int
		err = rows.Scan(&blobstoreSize, &cfPushTime)
		if err != nil {
			log.Fatalf("Failed to read results: %s", err.Error())
		}
		blobstoreSizes = append(blobstoreSizes, float64(blobstoreSize)/1024)
		cfPushTimes = append(cfPushTimes, float64(cfPushTime)/60)
	}

	pts := getPoints(blobstoreSizes, cfPushTimes)

	p, err := plot.New()
	if err != nil {
		panic(err)
	}

	p.Title.Text = "Experiment metrics"
	p.X.Label.Text = "Blobstore size (GB)"
	p.Y.Label.Text = "CF push time (min)"

	err = plotutil.AddLines(p,
		"versioned", pts,
	)
	if err != nil {
		panic(err)
	}

	err = p.Save(4*vg.Inch, 4*vg.Inch, os.Getenv("METRICS_FILE"))
	if err != nil {
		panic(err)
	}
}

func getPoints(Xs, Ys []float64)  plotter.XYs {
	n := len(Xs)
	pts := make(plotter.XYs, n)
	for i := 0; i < n; i++ {
		pts[i].X = Xs[i]
		pts[i].Y = Ys[i]
	}
	return pts
}

