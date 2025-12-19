// Bootstraps the Go WebAssembly module for the WaterColorMap playground.
//
// Requires `wasm_exec.js` to be loaded first (defines globalThis.Go).
//
// This script loads `wasm.wasm`, starts the Go runtime, and then signals readiness
// by setting `globalThis.__watercolormapWasmReady = true` and dispatching a
// `watercolormap:wasm-ready` event.

(function () {
  const statusEl = document.getElementById("status");

  function setStatus(msg) {
    if (statusEl) statusEl.textContent = msg;
    console.log(`[WASM] ${msg}`);
  }

  function dispatchReady() {
    globalThis.__watercolormapWasmReady = true;
    window.dispatchEvent(new CustomEvent("watercolormap:wasm-ready"));
  }

  async function instantiateGoWasm(url) {
    const go = new Go();

    // Prevent stale caching (especially on GitHub Pages) when updating wasm.wasm.
    // This keeps iteration smoother without requiring users to hard-refresh.
    const fetchWasm = (u) => fetch(u, { cache: "no-store" });

    // GitHub Pages typically serves .wasm with application/wasm, but local servers
    // might not; we fall back to ArrayBuffer instantiation.
    if (WebAssembly.instantiateStreaming) {
      try {
        const result = await WebAssembly.instantiateStreaming(
          fetchWasm(url),
          go.importObject,
        );
        return { go, instance: result.instance };
      } catch (err) {
        console.warn("[WASM] instantiateStreaming failed, falling back:", err);
      }
    }

    const resp = await fetchWasm(url);
    const bytes = await resp.arrayBuffer();
    const result = await WebAssembly.instantiate(bytes, go.importObject);
    return { go, instance: result.instance };
  }

  async function waitForExports(timeoutMs = 3000) {
    const start = Date.now();
    while (Date.now() - start < timeoutMs) {
      if (
        typeof globalThis.watercolorInit === "function" &&
        typeof globalThis.watercolorGenerateTile === "function"
      ) {
        return true;
      }
      await new Promise((r) => setTimeout(r, 50));
    }
    return false;
  }

  async function main() {
    try {
      if (typeof Go === "undefined") {
        setStatus("Error: wasm_exec.js not loaded");
        return;
      }

      setStatus("Loading WASM runtime...");
      const { go, instance } = await instantiateGoWasm("wasm.wasm");

      setStatus("Starting Go runtime...");
      // Do not await: main blocks forever (by design), but exports become available.
      go.run(instance);

      const ok = await waitForExports();
      if (!ok) {
        setStatus("Error: WASM exports not ready");
        return;
      }

      try {
        globalThis.watercolorInit();
      } catch (err) {
        console.warn("[WASM] watercolorInit failed:", err);
      }

      setStatus("WASM ready");
      dispatchReady();
    } catch (err) {
      console.error("[WASM] Failed to bootstrap:", err);
      setStatus(`Error: ${err && err.message ? err.message : String(err)}`);
    }
  }

  // Kick off immediately.
  main();
})();
