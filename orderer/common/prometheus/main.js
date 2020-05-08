'use strict';

const PrometheusQueryClient = require('./prometheus-query-client')

class Main {
    constructor() {
        this.prometheusQueryClient = new PrometheusQueryClient("http://localhost:9090/api/v1/query?query=ledger_blockchain_height&time=1588952871.162&_=1588952858317");
        this.prometheusQueryClient.testUrl();
        this.prometheusQueryClient.retrieveTargetParameters("query", "ledger_blockchain_height");
    }

    getQueryClient() {
        return this.prometheusQueryClient;
    }
}

module.exports = Main;