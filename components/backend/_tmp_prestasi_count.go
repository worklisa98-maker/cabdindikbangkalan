package main

import (
  "context"
  "fmt"
  "os"

  "cabdin-bangkalan-backend/internal/services"
)

func main() {
  b, err := os.ReadFile("testdata/prestasi-import/dummy-achievement-import-test.xlsx")
  if err != nil { panic(err) }
  resp, err := services.ValidatePrestasiImport(context.Background(), b, 1, services.ValidateOptions{IgnoreIDPrestasi: true}, nil)
  if err != nil { panic(err) }
  fmt.Printf("total=%d accepted=%d rejected=%d\n", resp.Summary.TotalRows, resp.Summary.AcceptedCount, resp.Summary.RejectedCount)
}
