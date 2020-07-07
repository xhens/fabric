package prometheus

import "fmt"

func RunControllerService() {

	/*	defer func() {
		ticker := time.NewTicker(1)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				controller.Run()
			}
		}
	}()*/
}

func Main() {
	fmt.Println("Starting to run the controller service...")
	// deliverBlocksSent := MetricMonitor{Metric: DeliverBLocksSent, MetricType: Matrix, Label: BlockDataType, StatType: Max}
	// RunControllerService()

	// ledgerTransactionCount := MetricMonitor{Metric: LedgerTransactionCount, MetricType: Matrix, Label: Chaincode, StatType: Max}
	//defer controller.Run()
	// deliverBlocksSent.RunOnce()

	// TODO: Get values and make the controller
	// TODO: Evaluation
	// TODO: High throughput without controller with different configurations -> find the best block size, or saturation point,
	// TODO: Write about implementation

	//firstMetric := make(chan MetricMonitor)
	//secondMetric := make(chan MetricMonitor)
	//valHolder := make(chan float64)
	//valHolder2 := make(chan float64)
	/*	defer func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				fmt.Println("just a print")
				// firstMetric := make(chan MetricMonitor)


				// firstMetricValue := <- firstMetric
				// fmt.Println(firstMetricValue.Value)
			}
		}
	}()*/
}