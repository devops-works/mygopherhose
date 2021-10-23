package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/term"

	_ "github.com/go-sql-driver/mysql"
)

const BUFSIZE = 10 * 1024 * 1024

func main() {
	var (
		wg                               sync.WaitGroup
		host, user, pass, port, database string
		procs, bufsize                   int
	)

	flag.StringVar(&host, "h", "127.0.0.1", "database host")
	flag.StringVar(&user, "u", "", "database user")
	flag.StringVar(&pass, "p", "", "database pass")
	flag.StringVar(&database, "d", "", "database name")
	flag.StringVar(&port, "P", "", "database port")
	flag.IntVar(&bufsize, "b", BUFSIZE, "buffer size")
	flag.IntVar(&procs, "c", 20, "concurrency")

	flag.Parse()

	if flag.NArg() != 1 {
		usage()
		os.Exit(1)
	}

	// fmt.Println(flag.Args())
	file := flag.Args()[0]

	if pass == "" {
		fmt.Printf("Password: ")
		bytes, err := term.ReadPassword(syscall.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println()

		pass = string(bytes)
	}

	fmt.Println("Host        :", host)
	fmt.Println("User        :", user)
	fmt.Println("Database    :", database)
	fmt.Println("Buffer size :", bufsize, "bytes")
	fmt.Println("File        :", file)

	f, err := os.Open(file)
	if err != nil {
		log.Fatalf("cannot open slow query log file: %s", err)
	}
	defer f.Close()

	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?multiStatements=true", user, pass, host, database))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// check access
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	c := make(chan []byte, procs*4)

	for i := 0; i < procs; i++ {
		wg.Add(1)
		go worker(&wg, db, c)
	}

	// Prepare scanner & its buffer
	s := *bufio.NewScanner(f)

	// set buffer size to bufsize
	buf := make([]byte, 0, bufsize)
	// allow scanner to allocate up to bufsize
	// by default bufio.MaxScanTokenSize (65536) is used
	s.Buffer(buf, bufsize)

	err = scan(s, db, c)
	if err != nil {
		fmt.Printf("error returned from scanner: %v\n", err)
	}
	close(c)

	fmt.Printf("\nWaiting for goroutines...\n")
	wg.Wait()
}

func usage() {
	fmt.Printf("%s [-h host] -u user -p [password] [-P port] [-d dbname] [-b bufsize] dumpfile\n", os.Args[0])
	fmt.Printf("\t-h defaults to 127.0.0.1\n\t-P defaults to 3306\n\t-b defaults to %d bytes\n", 10*1024*1024)
	fmt.Printf("\t-d can be omitted is dump contains `use database;` stanza\n")
	fmt.Printf("\t-p if parameter is empty, password will be asked interactively\n")
}

func worker(wg *sync.WaitGroup, db *sql.DB, c <-chan []byte) {
	defer wg.Done()

	for sql := range c {
		_, err := db.Exec("SET FOREIGN_KEY_CHECKS=0;" + string(sql))
		fmt.Printf(".")
		if err != nil {
			fmt.Printf("\n%s: %v\n", string(sql), err)
		}
	}
}

func scan(s bufio.Scanner, db *sql.DB, c chan<- []byte) error {
	var (
		accum []byte
		count int
	)

	for s.Scan() {
		b := s.Bytes()

		if len(b) == 0 || b[0] == '-' || b[0] == '/' {
			continue
		}

		accum = append(accum, b...)

		if b[len(b)-1] != ';' {
			continue
		}

		switch string(accum[0:4]) {
		case "LOCK":
			// 	fmt.Printf("🔒")
			accum = nil
			continue
		case "UNLO":
			fmt.Printf("⌛")
			accum = nil
			continue
		case "INSE":
			// dispatch INSERTS below
			c <- accum
			accum = nil
		default:
			// execute the rest (CREATE/DROP TABLE) locally
			// but wait for query chan to be empty before
			// for {
			// 	if len(c) == 0 {
			// 		break
			// 	}
			// 	// fmt.Print("<", len(c), ">")
			// 	time.Sleep(100 * time.Millisecond)
			// }

			cmd := strings.Split(string(accum), " ")

			if strings.ToUpper(cmd[0]) == "CREATE" {
				fmt.Printf("%d statements\n🐑 %s", count, strings.Trim(cmd[2], "`"))
				count = 0
			}
			_, err := db.Exec("SET FOREIGN_KEY_CHECKS=0;" + string(accum))
			if err != nil {
				fmt.Printf("\n%s: %v\n", string(accum), err)
			}
			accum = nil
			continue
		}

		// dispatch in channel
		c <- accum
		count++
		accum = nil
	}

	return s.Err()
}
