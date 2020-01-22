# go-bf-rankers
This program is get bitflyer ranking, and save to levelDB.
Output data if input console name and start, end date.

# Usage
```
$ git clone https://github.com/go-numb/go-bf-rankers.git
$ cd go-bf-rankers
$ go get && go build
$ mkdir logs
$ ./go-bf-rankers &
// works loop for get&save each 15 minutes.

// ex.) input scan console
$ User123456 20200120 20200122
Name - 0.19/1約定平均...
Name - 0.19/1約定平均...
Name - 0.19/1約定平均...
Name - 0.19/1約定平均...
```

# Author
[_numbP](https://twitter.com/_numbP)