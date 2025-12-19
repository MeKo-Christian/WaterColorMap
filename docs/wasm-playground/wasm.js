// Simple semaphore for limiting concurrent operations
class Semaphore {
  constructor(max) {
    this.max = max;
    this.count = 0;
    this.queue = [];
  }

  async acquire() {
    if (this.count < this.max) {
      this.count++;
      return;
    }
    await new Promise((resolve) => this.queue.push(resolve));
    this.count++;
  }

  release() {
    this.count--;
    if (this.queue.length > 0) {
      const next = this.queue.shift();
      next();
    }
  }
}

// Get concurrency from WASM or fallback to navigator.hardwareConcurrency or 4
function getMaxConcurrency() {
  // Try WASM function first
  if (typeof watercolorGetConcurrency === "function") {
    try {
      const cores = watercolorGetConcurrency();
      if (typeof cores === "number" && cores > 0) {
        return cores;
      }
    } catch (e) {
      // fall through
    }
  }
  // Fallback to browser API
  if (navigator.hardwareConcurrency) {
    return navigator.hardwareConcurrency;
  }
  // Default
  return 4;
}

// Demo UI Manager
class WaterColorMapPlayground {
  constructor() {
    this.map = null;
    this.tileLayer = null;
    this.statusEl = document.getElementById("status");
    this.maxConcurrency = getMaxConcurrency();
    this.fetchSemaphore = new Semaphore(this.maxConcurrency);
    this.overpassEndpoint = "https://overpass-api.de/api/interpreter";
    this.init();
  }

  async init() {
    // Initialize map
    const hanoverCenter = [52.375, 9.732];
    this.map = L.map("map", {
      center: hanoverCenter,
      zoom: 13,
      minZoom: 10,
      maxZoom: 16,
    });

    this.tileLayer = this.createGridLayer();

    this.tileLayer.addTo(this.map);

    this.updateStatus(
      `Ready. In-browser rendering via Overpass (${this.maxConcurrency} CPUs)`
    );
  }

  createGridLayer() {
    const self = this;
    const WaterColorGridLayer = L.GridLayer.extend({
      createTile(coords, done) {
        const img = document.createElement("img");
        img.alt = "";
        img.setAttribute("role", "presentation");

        const dpr = window.devicePixelRatio || 1;
        const is2x = dpr >= 2;
        const z = coords.z;
        const x = coords.x;
        const y = coords.y;

        self
          .loadTileToImg({ z, x, y, is2x, img })
          .then(() => done(null, img))
          .catch((err) => {
            console.warn("tile load failed", err);
            done(err, img);
          });

        return img;
      },
    });

    return new WaterColorGridLayer({
      attribution: "© OpenStreetMap contributors | WaterColorMap Playground",
      tileSize: 256,
      maxZoom: 16,
      minZoom: 10,
      updateWhenIdle: true,
      updateWhenZooming: false,
      keepBuffer: 2,
    });
  }

  async fetchOverpassJSON(query) {
    const resp = await fetch(this.overpassEndpoint, {
      method: "POST",
      mode: "cors",
      body: query,
    });
    if (!resp.ok) {
      throw new Error(`Overpass HTTP ${resp.status}`);
    }
    return await resp.text();
  }

  async loadTileToImg({ z, x, y, is2x, img }) {
    if (typeof watercolorOverpassQueryForTile !== "function") {
      img.src = this.makePlaceholderDataUrl("WASM not ready");
      this.updateStatus("WASM not ready");
      return;
    }
    if (typeof watercolorRenderTileFromOverpassJSON !== "function") {
      img.src = this.makePlaceholderDataUrl("WASM API missing");
      this.updateStatus("WASM API missing");
      return;
    }

    // Fetch from backend with concurrency limit
    await this.fetchSemaphore.acquire();
    try {
      try {
        const req = { zoom: z, x, y, hidpi: is2x };
        const q = watercolorOverpassQueryForTile(JSON.stringify(req));
        if (!q || !q.query) {
          throw new Error(q && q.error ? q.error : "query build failed");
        }

        this.updateStatus(
          `Overpass z${z} ${x}/${y}... (${this.maxConcurrency} concurrent)`
        );

        const overpassJSON = await this.fetchOverpassJSON(q.query);
        const rendered = watercolorRenderTileFromOverpassJSON(
          JSON.stringify(req),
          overpassJSON
        );
        if (!rendered || !rendered.pngBase64) {
          throw new Error(
            rendered && rendered.error ? rendered.error : "render failed"
          );
        }

        img.src = `data:${rendered.mime || "image/png"};base64,${
          rendered.pngBase64
        }`;
        if (typeof rendered.ms === "number") {
          this.updateStatus(
            `Rendered z${z} ${x}/${y} in ${rendered.ms}ms (${this.maxConcurrency} concurrent)`
          );
        }
      } catch (err) {
        img.src = this.makePlaceholderDataUrl("Render error");
        this.updateStatus(String(err && err.message ? err.message : err));
        return;
      }
    } finally {
      this.fetchSemaphore.release();
    }
  }

  makePlaceholderDataUrl(message) {
    const svg = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="256" height="256">
  <rect width="100%" height="100%" fill="#f5f5f5"/>
  <text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle" font-family="sans-serif" font-size="14" fill="#777">${String(
    message
  )
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")}</text>
</svg>`;
    return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
  }

  updateStatus(msg) {
    if (this.statusEl) {
      this.statusEl.textContent = msg;
    }
    console.log(`[Status] ${msg}`);
  }
}

// Initialize on page load
window.addEventListener("load", () => {
  window.playground = new WaterColorMapPlayground();
});
