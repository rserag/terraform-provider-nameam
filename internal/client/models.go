package client

type DomainsResponse struct {
	Docs        []Domain `json:"docs"`
	TotalDocs   int      `json:"totalDocs"`
	Limit       int      `json:"limit"`
	Page        int      `json:"page"`
	TotalPages  int      `json:"totalPages"`
	HasPrevPage bool     `json:"hasPrevPage"`
	HasNextPage bool     `json:"hasNextPage"`
	PrevPage    *int     `json:"prevPage"`
	NextPage    *int     `json:"nextPage"`
}

type Domain struct {
	ID          string       `json:"_id"`
	Domain      string       `json:"domain"`
	DnsUsed     bool         `json:"dnsUsed"`
	NameServers []NameServer `json:"nameServers"`
	Records     []DNSRecord  `json:"records"`
}

type NameServer struct {
	Hostname string `json:"hostname"`
}

type DNSRecord struct {
	ID      string `json:"id"`
	Access  bool   `json:"access"`
	Proxied bool   `json:"proxied"`
	Type    string `json:"type"`
	TTL     int64  `json:"ttl"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type UpdateDomainRequest struct {
	Records []UpdateRecord `json:"records"`
}

type UpdateRecord struct {
	// For CREATE
	Type    string `json:"type,omitempty"`
	TTL     int64  `json:"ttl,omitempty"`
	Prefix  string `json:"prefix,omitempty"`
	Name    string `json:"name,omitempty"`
	Content string `json:"content,omitempty"`

	// For DELETE
	ID string `json:"id,omitempty"`

	// CREATE | UPDATE | DELETE
	Action string `json:"action"`
}
