// denu run --allow-env --allow-net applylabels.js

import { serve } from "https://deno.land/std@0.83.0/http/server.ts";

const SERVICE_URL =
  Deno.env.get("AE_SERVICE_URL") || "http://app-exposer/vice/apply-labels";
const HOST = Deno.env.get("AE_HOSTNAME") || "0.0.0.0";
const PORT = parseInt(Deno.env.get("AE_PORT"), 10) || 60000;
const INTERVAL = parseInt(Deno.env.get("AE_INTERVAL"), 10) || 10000;

const applyLabels = async () => {
  const response = await fetch(SERVICE_URL, {
    method: "POST",
    cache: "no-cache",
  });

  const msg = await response.text();
  const d = new Date();

  console.log(
    `${d.toLocaleDateString()} ${d.toLocaleTimeString()} GET ${SERVICE_URL} ${
      response.status
    } ${response.statusText} ${msg}`
  );
};

const server = serve({ hostname: HOST, port: PORT });
console.log(`HTTP server is running at http://localhost:60000/`);

setInterval(applyLabels, INTERVAL);

for await (const request of server) {
  request.respond({ status: 200, body: "Hello from applylabels!" });
}
