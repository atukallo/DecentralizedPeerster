package gossiper

import "time"

const (
	RumorTimeout       = 1 * time.Second // if peer doesn't answer with status, flip a coin
	AntiEntropyTimeout = 1 * time.Second // send statuses every time timeout shoots

	FileDownloadReplyTimeout  = 5 * time.Second // if peer doesn't answer with chunk, resend the chunk request
	FileDownloadTimeoutsLimit = 5               // number of times requests are resent

	FileSearchStartBudget          = 2 // if budget is not specified in cli, it's gradually increased till reaches max
	FileSearchMaxBudget            = 32
	FileSearchReplyTimeout         = 1 * time.Second // if didn't get threshold matches, resend the search-request
	FileSearchFullMatchesThreshold = 2               // should find threshold peers with !all! the chunks to stop increasing budget

	RecentSearchRequestTimeout = 500 * time.Millisecond // don't answer to same search-requests for some time

	DefaultHopLimit = 5 // used eg for private messages, search replies, data replies, etc..
)
