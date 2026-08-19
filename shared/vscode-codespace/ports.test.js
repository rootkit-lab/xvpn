"use strict";

const { test } = require("node:test");
const assert = require("node:assert/strict");
const { parseProcNetTcp, parseSsOutput, isDemoBind, isDemoHost, isDemoPreviewUrl, previewUrl } = require("./ports");

test("isDemoBind aceita 0.0.0.0 e docker0", () => {
  assert.equal(isDemoBind([0, 0, 0, 0]), true);
  assert.equal(isDemoBind([172, 17, 0, 2]), true);
  assert.equal(isDemoBind([127, 0, 0, 1]), false);
});

test("parseProcNetTcp encontra Flask em 0.0.0.0:8080", () => {
  const raw = [
    "sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode",
    "   0: 00000000:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000    0        0 123 1 000 100 0 0 10 0",
    "   1: 0100007F:0BB8 00000000:0000 0A 00000000:00000000 00:00000000 00000000    0        0 124 1 000 100 0 0 10 0",
  ].join("\n");
  const ports = parseProcNetTcp(raw);
  assert.deepEqual(ports, [{ port: 8080, addr: "0.0.0.0", public: true }]);
});

test("preview só aceita http://demo-*.corp.ihuull.com:porta", () => {
  assert.equal(isDemoHost("demo-cs-125848439153.corp.ihuull.com"), true);
  assert.equal(isDemoHost("evil.com"), false);
  assert.equal(isDemoHost('demo-x.corp.ihuull.com"><img src=x>'), false);
  assert.equal(previewUrl("demo-cs-125848439153.corp.ihuull.com", 8080), "http://demo-cs-125848439153.corp.ihuull.com:8080");
  assert.equal(isDemoPreviewUrl("http://demo-cs-125848439153.corp.ihuull.com:8080"), true);
  assert.equal(isDemoPreviewUrl("https://demo-cs-125848439153.corp.ihuull.com:8080"), false);
  assert.equal(isDemoPreviewUrl("http://evil.example:8080"), false);
  assert.equal(isDemoPreviewUrl("http://demo-cs-125848439153.corp.ihuull.com:8080/phish"), false);
  assert.equal(isDemoPreviewUrl("https://evil.com"), false);
});

test("parseSsOutput encontra 0.0.0.0:8080", () => {
  const ports = parseSsOutput("LISTEN 0      128          0.0.0.0:8080       0.0.0.0:*\n");
  assert.equal(ports.length, 1);
  assert.equal(ports[0].port, 8080);
  assert.equal(ports[0].public, true);
});
