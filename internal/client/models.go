package client

type DomainsResponse struct {
	Docs  []Domain `json:"docs"`
	Total int      `json:"total"`
	Limit int      `json:"limit"`
	Page  int      `json:"page"`
	Pages int      `json:"pages"`
}

type Domain struct {
	ID        string      `json:"id"`
	Domain    string      `json:"domain"`
	DnsUsed   bool        `json:"dnsUsed"`
	NameServers []NameServer `json:"nameServers"`
	Records   []DNSRecord `json:"records"`
}

type NameServer struct {
	Hostname string `json:"hostname"`
}

type DNSRecord struct {
	ID      string `json:"id"`
	Access  bool   `json:"access"`
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

	// CREATE | DELETE
	Action string `json:"action"`
}
