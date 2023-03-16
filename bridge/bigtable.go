package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"

	"cloud.google.com/go/bigtable"
)

const (
	projectID        = "my-project"
	instanceID       = "my-instance"
	tableName        = "user"
	columnFamilyName = "daily"
	columnQualifier  = "temp"
)

var client *bigtable.Client

func init() {
	var err error
	client, err = bigtable.NewClient(context.TODO(), projectID, instanceID)
	if err != nil {
		log.Fatal(err)
	}
}

func write(ctx context.Context, rowKey string) error {
	tbl := client.Open(tableName)

	mut := bigtable.NewMutation()
	mut.Set(columnFamilyName, columnQualifier, bigtable.Timestamp(0), []byte(genTemp()))

	if err := tbl.Apply(ctx, rowKey, mut); err != nil {
		return err
	}

	return nil
}

func genTemp() string {
	rand.Seed(time.Now().UnixNano())
	return strconv.Itoa(rand.Intn(30))
}

func readRows(ctx context.Context, prefix string) error {
	tbl := client.Open(tableName)
	err := tbl.ReadRows(ctx, bigtable.PrefixRange(prefix),
		showRow,
		bigtable.RowFilter(
			bigtable.ChainFilters(
				bigtable.FamilyFilter(columnFamilyName),
				bigtable.ColumnFilter(columnQualifier),
			),
		),
		bigtable.LimitRows(3),
	)
	if err != nil {
		return err
	}

	return nil
}

func showRow(row bigtable.Row) bool {
	for _, columns := range row {
		for _, column := range columns {
			fmt.Printf("row: %s, column: %s, value: %s, timestamp: %d\n", column.Row, column.Column, string(column.Value), column.Timestamp)
		}
	}
	return true
}
