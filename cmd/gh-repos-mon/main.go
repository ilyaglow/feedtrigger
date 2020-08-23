package main

import (
	"context"
	"log"

	"ilya.app/feedtrigger"
)

func main() {
	feeds := []feedtrigger.Feed{
		*feedtrigger.NewFeed("https://github.com/Neo23x0/sigma/commits/master.atom", feedtrigger.LogAuthorAndLink),
		*feedtrigger.NewFeed("https://github.com/StrangerealIntel/DailyIOC/commits.atom", feedtrigger.LogAuthorAndLink),
		*feedtrigger.NewFeed("https://github.com/blackorbird/APT_REPORT/commits.atom", feedtrigger.LogAuthorAndLink),
	}

	app, err := feedtrigger.New(nil, feeds...)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(app.Run(context.Background()))
}
