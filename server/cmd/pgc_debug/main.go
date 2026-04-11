// pgc_debug 对指定 pgc_id 顺序请求若干公开 PGC 接口，用于本地/联调排障（需后端已启动，默认 localhost:9000）。
//
//	go run ./cmd/pgc_debug -base=http://127.0.0.1:9000 -pgc_id=<snowflake>
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	base := flag.String("base", "http://localhost:9000", "backend base url")
	pgcID := flag.String("pgc_id", "", "pgc_id (snowflake, pass as string)")
	flag.Parse()

	if *pgcID == "" {
		fmt.Fprintln(os.Stderr, "missing -pgc_id")
		os.Exit(2)
	}

	client := &http.Client{Timeout: 15 * time.Second}

	fmt.Println("== 1) /api/v1/pgc/detail-with-episodes ==")
	get(client, *base+"/api/v1/pgc/detail-with-episodes?pgc_id="+*pgcID)

	fmt.Println("\n== 2) /api/v1/pgc/detail ==")
	get(client, *base+"/api/v1/pgc/detail?pgc_id="+*pgcID)

	fmt.Println("\n== 3) /api/v1/pgc/:pgc_id/episodes (page_size=100) ==")
	get(client, *base+"/api/v1/pgc/"+*pgcID+"/episodes?page=1&page_size=100")

	fmt.Println("\n== 4) /api/v1/pgc/list (page_size=10) ==")
	get(client, *base+"/api/v1/pgc/list?page=1&page_size=10")
}

func get(client *http.Client, url string) {
	fmt.Println("GET", url)
	resp, err := client.Get(url)
	if err != nil {
		fmt.Println("ERR:", err)
		return
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	fmt.Println("HTTP", resp.StatusCode)

	// 尝试用 UseNumber 打印关键字段，避免本地调试也被 float64 精度坑到
	var v any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		fmt.Println(string(b))
		return
	}

	pretty, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(pretty))
}

