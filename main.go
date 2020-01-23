package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/go-numb/atCoder/snippet"

	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/go-numb/go-bitflyer/v1/hidden/ranking"
	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	"golang.org/x/sync/errgroup"
)

const (
	DBTABLERANKING = "rankers"
	layout         = "20060102"
	layoutN        = "2006/01/02"
	layoutISO      = "2006-01-02"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary
var f *os.File
var isScanf bool

func init() {
	flag.BoolVar(&isScanf, "c", false, "<-c> is use command <name start_date end_date>")
	flag.Parse()

	fmt.Printf(`
# %s
name 20200101 20200102
or
name 2020/01/01 2020/01/02
or
name 2020-01-01 2020-01-02

# ex.) Output
Name - 0.19/1約定平均...

`, color.RedString("Input support"))
}

type Client struct {
	db *leveldb.DB

	Input chan string

	Logger *log.Entry
}

func New() *Client {
	ldb, err := leveldb.OpenFile("ldb", nil)
	if err != nil {
		log.Fatal(err)
	}

	l := log.New()
	projectDir, err := os.Getwd()
	if err != nil {
		l.Fatal(err)
	}
	f, err = os.OpenFile(
		filepath.Join(projectDir, "logs", fmt.Sprintf("%s_error.log", time.Now().Format("02-01-2006"))),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0666)
	if err != nil {
		l.Fatal(err)
	}
	l.SetLevel(log.ErrorLevel)
	l.SetOutput(f)
	l.SetFormatter(&log.JSONFormatter{})

	return &Client{
		db:     ldb,
		Input:  make(chan string),
		Logger: log.NewEntry(l),
	}
}

func (p *Client) Close() error {
	if err := p.db.Close(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

func main() {
	client := New()
	defer client.Close()

	signal := make(chan os.Signal)

	if isScanf {
		go client.Wait()
	}
	fmt.Println("start program...")

	var eg errgroup.Group

	eg.Go(func() error {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		defer close(signal)

		for {
			select {
			case t := <-ticker.C:
				if t.Minute()%15 != 0 {
					continue
				}
				if err := client.Ranking(); err != nil {
					log.Error(err)
					continue
				}

			case s := <-client.Input:
				fmt.Println("input event: ", s)
				name, start, end, ok := toNameStartEnd(s)
				if !ok {
					client.Logger.Error("input data undefined", name, start, end)
					continue
				}
				rankers := client.Get(name, start, end)
				if rankers == nil {
					fmt.Println("has not data")
					continue
				}
				for i := range rankers {
					fmt.Printf("%s - %.1f - %.2f/1約定平均	%s\n", rankers[i].Nickname, rankers[i].Volume, rankers[i].Volume/float64(rankers[i].NumberOfTrades), rankers[i].CreatedAt.Format("2006/01/02 15:04"))
				}
				// 変数解放
				_ = rankers

			case sig := <-signal:
				return fmt.Errorf("get signal %v", sig)
			}
		}
	})

	if err := eg.Wait(); err != nil {
		log.Fatal(err)
	}
}

func (p *Client) Ranking() error {
	rankers, err := ranking.Get("VOLUME")
	if err != nil {
		return err
	}

	now := time.Now()
	for i := range rankers {
		rankers[i].CreatedAt = now
		b, err := json.Marshal(rankers[i])
		if err != nil {
			p.Logger.Error(err)
			continue
		}

		if err := p.db.Put([]byte(fmt.Sprintf("%s:%s:%d", DBTABLERANKING, rankers[i].Nickname, now.UnixNano())), b, nil); err != nil {
			p.Logger.Error(err)
			continue
		}
	}

	return nil
}

func (p *Client) Wait() {
	for {
		_, s := snippet.Scanf(3)
		go func() { p.Input <- s }()
	}
}

func toNameStartEnd(s string) (name string, start, end time.Time, ok bool) {
	str := strings.Split(s, " ")
	if str == nil || len(str) < 1 {
		return name, start, end, false
	}

	for i := range str {
		switch i {
		case 0:
			name = str[i]

		default:
			var (
				t   time.Time
				err error
			)
			t, err = time.Parse(layout, str[i])
			if err != nil { // 多段解析
				t, err = time.Parse(layoutN, str[i])
				if err != nil {
					t, err = time.Parse(layoutISO, str[i])
					if err != nil {
						t = time.Now()
					}
				}
			}

			switch i { // 強制的にBitflyer JSTへ
			case 1:
				start = t.Add(9 * time.Hour)
				ok = true
			case 2:
				end = t.Add(9 * time.Hour)
				ok = true
			default:
				ok = false
			}

		}
	}
	return name, start, end, ok
}

func (p *Client) Get(name string, start, end time.Time) []ranking.Ranker {
	rows := p.db.NewIterator(&util.Range{
		Start: []byte(fmt.Sprintf("%s:%s:%d", DBTABLERANKING, name, start.UnixNano())),
		Limit: []byte(fmt.Sprintf("%s:%s:%d", DBTABLERANKING, name, end.UnixNano())),
	}, nil)
	if rows == nil {
		return nil
	}

	var (
		ranker  ranking.Ranker
		rankers []ranking.Ranker
	)
	for rows.Next() {
		value := rows.Value()
		if err := json.Unmarshal(value, &ranker); err != nil {
			continue
		}
		rankers = append(rankers, ranker)
	}

	rows.Release()
	if rows.Error() != nil {
		return nil
	}

	return rankers
}
