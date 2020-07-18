'use strict';

const url = require('url');
const http = require('http');
const https = require('https');

const NULL = '/api/v1/';
const RANGE = '/api/v1/query_range';
const SINGLE = '/api/v1/query';

class PrometheusQueryClient {
    constructor(prometheusUrl) {
        this._prometheusUrl = prometheusUrl;
    }

    get prometheusUrl() {
        return this._prometheusUrl;
    }

    set prometheusUrl(prometheusUrl) {
        this._prometheusUrl = prometheusUrl;
    }

    testUrl() {
        console.log(this._prometheusUrl);
    }

    retrieveTargetParameters(type, query) {
        const target = url.resolve(this._prometheusUrl, type + query);
        console.log(target);
        console.log(typeof target)
        console.log(typeof this.prometheusUrl);
        const requestParams = url.parse(target);
        const httpModule = this.isHttps(requestParams.href) ? https: http;
        return {
            requestParams,
            httpModule
        }
    }

    async rangeQuery(queryString, startTime, endTime, step=1) {
        const query = '?query=' + queryString + '&start=' + startTime + '&end=' + endTime + '&step=' + step;
        console.log("Issuing range query: ", query);
        const targetParams = this.retrieveTargetParameters(RANGE, query);
        return await this.retrieveResponse(targetParams);
    }

    async retrieveResponse(targetParams) {
        try {
            const resp = await this.doRequest(targetParams);
            const response = JSON.parse(resp);
            if (response.status.localeCompare('success') === 0) {
                return response;
            } else {
                console.log(`Prometheus query to ${targetParams.requestParams.href} failed with: ${JSON.stringify(response)}`)
                return null
            }
        } catch (error) {
            console.log("Query error", error);
            throw error;
        }
    }

    doRequest(targetParams) {
        return new Promise((resolve, reject) => {
            const options = Object.assign(targetParams.requestParams, {
                method: 'GET',
                headers: {
                    'Content-Type': 'application/json'
                }
            });

            const req = targetParams.httpModule.request(options, res => {
                let body = '';
                res.setEncoding('utf8');
                res.on('data', chunk => {
                    body += chunk;
                })
                res.on('end', ()=>{
                    if (body) {
                        resolve(body);
                    } else {
                        resolve();
                    }
                });
                res.on('error', err => {
                    console.log(err);
                    reject(err);
                });
            });
            req.end();
        })
    }
}

module.exports = PrometheusQueryClient;