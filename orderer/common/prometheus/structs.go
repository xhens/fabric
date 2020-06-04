package prometheus

type KvPair struct {
	Name string
	Value float64
}

type KvMap struct {
	Items []KvPair
}

func (valuesMap *KvMap) AddItem(item KvPair) []KvPair {
	valuesMap.Items = append(valuesMap.Items, item)
	return valuesMap.Items
}

type InstantVectorObject struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric struct { // metric fields are not always the same. They depend on specific metrics
				Name     string `json:"__name__"`
				Channel  string `json:"channel"`
				Instance string `json:"instance"`
				Job      string `json:"job"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

type RangeVectorObject struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric struct {
				Name     string `json:"__name__"`
				Channel  string `json:"channel"`
				Instance string `json:"instance"`
				Job      string `json:"job"`
				Le  	 string `json:"le"`
			} `json:"metric"`
			Values [][]interface{} `json:"values"`
		} `json:"result"`
	} `json:"data"`
}