const map = L.map("map", {
  zoomControl: false,
  preferCanvas: true,
}).setView([34.007603, -118.499741], 17);

L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
  maxZoom: 20,
  attribution: "&copy; OpenStreetMap contributors",
}).addTo(map);

L.control.zoom({ position: "bottomleft" }).addTo(map);

const coverageLayer = L.layerGroup().addTo(map);
const coverageRenderer = L.canvas({ padding: 0.5 });

const els = {
  searchForm: document.getElementById("searchForm"),
  searchInput: document.getElementById("searchInput"),
  searchResults: document.getElementById("searchResults"),
  searchSpinner: document.getElementById("searchSpinner"),
	coverageToggle: document.getElementById("coverageToggle"),
	coverageCount: document.getElementById("coverageCount"),
	coveragePill: document.getElementById("coveragePill"),
	areaPrompt: document.getElementById("areaPrompt"),
  drawButton: document.getElementById("drawButton"),
  redrawButton: document.getElementById("redrawButton"),
  cancelDrawButton: document.getElementById("cancelDrawButton"),
  drawHint: document.getElementById("drawHint"),
  selectionCard: document.getElementById("selectionCard"),
  selectionTitle: document.getElementById("selectionTitle"),
  selectionSize: document.getElementById("selectionSize"),
  nameInput: document.getElementById("nameInput"),
  exportButton: document.getElementById("exportButton"),
  zoomInput: document.getElementById("zoomInput"),
  heightInput: document.getElementById("heightInput"),
  parallelInput: document.getElementById("parallelInput"),
  glbInput: document.getElementById("glbInput"),
  usdInput: document.getElementById("usdInput"),
  objInput: document.getElementById("objInput"),
  minecraftInput: document.getElementById("minecraftInput"),
  blockSizeInput: document.getElementById("blockSizeInput"),
  blockSizeField: document.getElementById("blockSizeField"),
  worldPath: document.getElementById("worldPath"),
  worldHint: document.getElementById("worldHint"),
  estimate: document.getElementById("estimate"),
  coordinateSummary: document.getElementById("coordinateSummary"),
  requestSummary: document.getElementById("requestSummary"),
  jobCard: document.getElementById("jobCard"),
  jobIcon: document.getElementById("jobIcon"),
  jobState: document.getElementById("jobState"),
  jobCount: document.getElementById("jobCount"),
  jobLog: document.getElementById("jobLog"),
  outputPath: document.getElementById("outputPath"),
	viewerLink: document.getElementById("viewerLink"),
  progressBar: document.getElementById("progressBar"),
  liveRegion: document.getElementById("liveRegion"),
  setupGate: document.getElementById("setupGate"),
  setupChecking: document.getElementById("setupChecking"),
  setupImport: document.getElementById("setupImport"),
  setupDropzone: document.getElementById("setupDropzone"),
  setupFileInput: document.getElementById("setupFileInput"),
  setupStatus: document.getElementById("setupStatus"),
  setupScanForm: document.getElementById("setupScanForm"),
  setupScanPath: document.getElementById("setupScanPath"),
};

let drawing = false;
let dragStart = null;
let selection = null;
let selectionBounds = null;
let locationMarker = null;
let pollTimer = null;
let searchTimer = null;
let searchController = null;
let reverseController = null;
let nameWasEdited = false;
let coverageTiles = [];
let coverageState = "loading";
let coverageVisible = true;
let coverageTimer = null;
let coverageController = null;
let exportRequestLimit = 200000;

els.setupDropzone.addEventListener("click", () => els.setupFileInput.click());
els.setupFileInput.addEventListener("change", () => {
  const [file] = els.setupFileInput.files;
  if (file) importSetupFile(file);
});

for (const eventName of ["dragenter", "dragover"]) {
  els.setupGate.addEventListener(eventName, (event) => {
    event.preventDefault();
    if (!els.setupImport.hidden) els.setupDropzone.classList.add("is-dragging");
  });
}

for (const eventName of ["dragleave", "drop"]) {
  els.setupGate.addEventListener(eventName, (event) => {
    event.preventDefault();
    els.setupDropzone.classList.remove("is-dragging");
  });
}

els.setupGate.addEventListener("drop", (event) => {
  if (els.setupImport.hidden) return;
  const [file] = event.dataTransfer.files;
  if (file) importSetupFile(file);
});

els.drawButton.addEventListener("click", startDrawing);
els.redrawButton.addEventListener("click", startDrawing);
els.cancelDrawButton.addEventListener("click", stopDrawing);
els.coverageToggle.addEventListener("click", () => {
  coverageVisible = !coverageVisible;
  if (coverageVisible) coverageLayer.addTo(map);
  else map.removeLayer(coverageLayer);
  els.coverageToggle.setAttribute("aria-pressed", String(coverageVisible));
  els.coverageToggle.classList.toggle("is-off", !coverageVisible);
  const icon = els.coverageToggle.querySelector("svg");
  if (icon) icon.setAttribute("data-lucide", coverageVisible ? "eye" : "eye-off");
  refreshIcons();
});

map.on("moveend", () => {
  window.clearTimeout(coverageTimer);
  coverageTimer = window.setTimeout(loadCoverage, 280);
});

map.on("mousedown", (event) => {
  if (!drawing) return;
  dragStart = event.latlng;
  if (selection) map.removeLayer(selection);
  selection = L.rectangle([dragStart, dragStart], {
    className: "leaflet-rectangle-selection",
    weight: 2,
		color: "#d9556e",
		fillColor: "#d9556e",
		fillOpacity: 0.13,
  }).addTo(map);
});

map.on("mousemove", (event) => {
  if (!drawing || !dragStart || !selection) return;
  selection.setBounds(L.latLngBounds(dragStart, event.latlng));
});

map.on("mouseup", (event) => {
  if (!drawing || !dragStart || !selection) return;
  selection.setBounds(L.latLngBounds(dragStart, event.latlng));
  selectionBounds = selection.getBounds();
  dragStart = null;
  stopDrawing();
  updateSelection();
  resolveSelectionName();
});

els.glbInput.addEventListener("change", updateFormats);
els.usdInput.addEventListener("change", updateFormats);
els.objInput.addEventListener("change", updateFormats);
els.minecraftInput.addEventListener("change", updateFormats);
els.zoomInput.addEventListener("change", updateSelection);
els.heightInput.addEventListener("input", updateSelection);
els.heightInput.addEventListener("change", loadCoverage);
els.nameInput.addEventListener("input", () => {
  nameWasEdited = true;
  updateExportButton();
});

els.searchInput.addEventListener("input", () => {
  window.clearTimeout(searchTimer);
  const query = els.searchInput.value.trim();
  if (query.length < 2) {
    closeSearchResults();
    return;
  }
  searchTimer = window.setTimeout(() => searchPlaces(query), 320);
});

els.searchForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const query = els.searchInput.value.trim();
  if (query.length < 2) return;
  const results = await searchPlaces(query);
  if (results && results.length === 1) choosePlace(results[0]);
});

document.addEventListener("click", (event) => {
  if (!event.target.closest(".search-wrap")) closeSearchResults();
});

els.exportButton.addEventListener("click", async () => {
  if (!selectionBounds || !els.nameInput.value.trim()) return;

  setExporting(true);
  els.jobCard.hidden = false;
  els.jobState.textContent = "Preparing export…";
  els.jobCount.textContent = "No files written yet";
  els.jobLog.textContent = "Sending the selected area to the exporter.";
  els.outputPath.hidden = true;
	els.viewerLink.hidden = true;
	els.worldPath.hidden = true;
	els.worldHint.hidden = true;
  els.progressBar.classList.remove("done", "error");
  updateJobProgress(0, estimateRequests(requestFromSelection()).requests);
  els.jobIcon.className = "job-icon is-spinning";
  els.jobIcon.innerHTML = '<i data-lucide="loader-circle" aria-hidden="true"></i>';
  refreshIcons();

  try {
    const response = await fetch("/api/export", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(requestFromSelection()),
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || "The export could not be started.");
    pollJob(data.id);
  } catch (error) {
    showJobError(error.message);
  }
});

function startDrawing() {
  drawing = true;
  dragStart = null;
  map.dragging.disable();
  map.getContainer().style.cursor = "crosshair";
  els.drawHint.hidden = false;
  els.drawButton.classList.add("active");
  els.liveRegion.textContent = "Drawing mode on. Drag across the map to select an area.";
}

function stopDrawing() {
  drawing = false;
  dragStart = null;
  map.dragging.enable();
  map.getContainer().style.cursor = "";
  els.drawHint.hidden = true;
  els.drawButton.classList.remove("active");
}

function requestFromSelection() {
  return {
    name: els.nameInput.value.trim(),
    north: selectionBounds.getNorth(),
    south: selectionBounds.getSouth(),
    east: selectionBounds.getEast(),
    west: selectionBounds.getWest(),
    zoom: Number(els.zoomInput.value),
    tryH: Number(els.heightInput.value),
    parallel: els.parallelInput.checked,
    glb: els.glbInput.checked,
    usd: els.usdInput.checked,
    obj: els.objInput.checked,
    minecraft: els.minecraftInput.checked,
    blockSize: Number(els.blockSizeInput.value),
  };
}

function updateFormats() {
  // At least one format has to stay on, otherwise there is nothing to export.
  const formats = [els.glbInput, els.usdInput, els.objInput, els.minecraftInput];
  if (!formats.some((input) => input.checked)) els.glbInput.checked = true;
  els.blockSizeField.hidden = !els.minecraftInput.checked;
  for (const input of formats) {
    input.closest(".format-card").classList.toggle("is-on", input.checked);
  }
  setExportLabel();
  updateExportButton();
}

function exportLabel() {
  const names = [
    [els.glbInput, "GLB"],
    [els.usdInput, "USDZ"],
    [els.objInput, "OBJ"],
    [els.minecraftInput, "Minecraft"],
  ]
    .filter(([input]) => input.checked)
    .map(([, name]) => name);
  return names.length > 2 ? `Export ${names.length} formats` : `Export ${names.join(" + ")}`;
}

function setExportLabel() {
  els.exportButton.querySelector("span").textContent = exportLabel();
}

function updateSelection() {
  if (!selectionBounds) {
    updateExportButton();
    return;
  }

  const req = requestFromSelection();
  const estimate = estimateRequests(req);
  const size = selectionSize(req);
  const tooLarge = estimate.requests > exportRequestLimit;
	const coverage = selectionCoverage(req);

  els.selectionCard.hidden = false;
	els.drawButton.hidden = true;
  els.selectionSize.textContent = `${formatDistance(size.width)} × ${formatDistance(size.height)}`;
  els.coordinateSummary.textContent = `${req.north.toFixed(5)}, ${req.west.toFixed(5)} → ${req.south.toFixed(5)}, ${req.east.toFixed(5)}`;
  els.requestSummary.textContent = `${estimate.tiles.toLocaleString()} map tiles · up to ${estimate.requests.toLocaleString()} lookups`;
	els.coveragePill.className = `coverage-pill is-${coverage.type}`;
	els.coveragePill.textContent = coverage.label;
	els.estimate.classList.toggle("error", tooLarge || coverage.type === "none");
	els.estimate.classList.toggle("warn", coverage.type === "partial" || coverage.type === "loading" || coverage.type === "unknown");
	if (tooLarge) {
		els.estimate.textContent = `This area needs ${estimate.requests.toLocaleString()} lookups; the current limit is ${exportRequestLimit.toLocaleString()}. Draw a smaller area, lower the detail, or raise ORBIS_MAX_REQUESTS.`;
	} else if (coverage.type === "none") {
		els.estimate.textContent = "No 3D data here. Move the selection into a green area.";
	} else if (coverage.type === "partial") {
		els.estimate.textContent = "Part of this selection is outside 3D coverage. Make it smaller for a complete export.";
	} else if (coverage.type === "loading") {
		els.estimate.textContent = "Checking this area for 3D coverage…";
	} else if (coverage.type === "unknown") {
		els.estimate.textContent = "Apple has no coverage index here. The export can continue and will verify each tile.";
	} else {
		els.estimate.textContent = `Ready to export ${formatDistance(size.width)} × ${formatDistance(size.height)}.`;
	}
  updateExportButton();
}

async function resolveSelectionName() {
  const center = selectionBounds.getCenter();
  reverseController?.abort();
  reverseController = new AbortController();
  els.selectionTitle.textContent = "Finding the place name…";
  els.nameInput.disabled = false;
  els.nameInput.placeholder = "Name this export";

  try {
    const response = await fetch(`/api/place-name?lat=${center.lat}&lon=${center.lng}`, {
      signal: reverseController.signal,
    });
    const place = await response.json();
    if (!response.ok) throw new Error(place.error);
    els.selectionTitle.textContent = place.name;
    if (!nameWasEdited || !els.nameInput.value.trim()) {
      els.nameInput.value = `${place.name} — ${today()}`;
      nameWasEdited = false;
    }
  } catch (error) {
    if (error.name === "AbortError") return;
    els.selectionTitle.textContent = "Selected map area";
    if (!nameWasEdited || !els.nameInput.value.trim()) {
      els.nameInput.value = `Map area — ${today()}`;
      nameWasEdited = false;
    }
  }
  updateExportButton();
}

async function searchPlaces(query) {
  searchController?.abort();
  searchController = new AbortController();
  els.searchSpinner.hidden = false;
  els.searchResults.hidden = true;

  try {
    const response = await fetch(`/api/search?q=${encodeURIComponent(query)}`, {
      signal: searchController.signal,
    });
    const results = await response.json();
    if (!response.ok) throw new Error(results.error || "Search is unavailable.");
    renderSearchResults(results, query);
    return results;
  } catch (error) {
    if (error.name === "AbortError") return null;
    renderSearchMessage("Search is unavailable right now. You can still move the map manually.");
    return null;
  } finally {
    els.searchSpinner.hidden = true;
  }
}

function renderSearchResults(results, query) {
  els.searchResults.replaceChildren();
  if (!results.length) {
    renderSearchMessage(`No places found for “${query}”.`);
    return;
  }

  results.forEach((place) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "search-result";
    button.setAttribute("role", "option");

    const name = document.createElement("strong");
    name.textContent = place.name;
		const detailRow = document.createElement("span");
		detailRow.className = "search-result-detail";
		const detail = document.createElement("span");
		detail.textContent = place.displayName;
		detailRow.append(detail);
		const coverageTile = coverageTileAt(place.lat, place.lon);
		if (coverageState === "ready" && coverageTile) {
			const hasCoverage = coverageTile.available;
			const badge = document.createElement("em");
			badge.className = `search-coverage ${hasCoverage ? "has-coverage" : "no-coverage"}`;
			badge.textContent = hasCoverage ? "3D" : "No 3D";
			detailRow.append(badge);
		}
		button.append(name, detailRow);
    button.addEventListener("click", () => choosePlace(place));
    els.searchResults.append(button);
  });
  els.searchResults.hidden = false;
  els.searchInput.setAttribute("aria-expanded", "true");
}

function renderSearchMessage(message) {
  els.searchResults.replaceChildren();
  const copy = document.createElement("p");
  copy.className = "search-message";
  copy.textContent = message;
  els.searchResults.append(copy);
  els.searchResults.hidden = false;
  els.searchInput.setAttribute("aria-expanded", "true");
}

function choosePlace(place) {
  map.flyTo([place.lat, place.lon], 17, { duration: 0.65 });
  if (locationMarker) map.removeLayer(locationMarker);
  locationMarker = L.circleMarker([place.lat, place.lon], {
    radius: 7,
    color: "#ffffff",
    weight: 3,
    fillColor: "#d9556e",
    fillOpacity: 1,
  }).addTo(map);
	els.searchInput.value = place.name;
	els.areaPrompt.textContent = `Checking ${place.name} for confirmed 3D tiles…`;
  closeSearchResults();
  els.liveRegion.textContent = `Map moved to ${place.name}.`;
}

function closeSearchResults() {
  els.searchResults.hidden = true;
  els.searchInput.setAttribute("aria-expanded", "false");
}

function updateExportButton() {
  let enabled = Boolean(selectionBounds && els.nameInput.value.trim());
	if (enabled) {
		const req = requestFromSelection();
		const coverage = selectionCoverage(req).type;
		enabled = estimateRequests(req).requests <= exportRequestLimit && (coverage === "full" || coverage === "unknown");
	}
  els.exportButton.disabled = !enabled;
}

async function loadCoverage() {
	coverageController?.abort();
	coverageController = new AbortController();
	coverageState = "loading";
	els.coverageCount.textContent = "Checking visible tiles…";
	if (selectionBounds) updateSelection();
  try {
		const bounds = map.getBounds();
		const zoom = Math.min(18, Math.max(15, Math.round(map.getZoom())));
		const params = new URLSearchParams({
			north: bounds.getNorth().toString(),
			south: bounds.getSouth().toString(),
			east: bounds.getEast().toString(),
			west: bounds.getWest().toString(),
			zoom: zoom.toString(),
			height: Math.min(64, Math.max(24, Number(els.heightInput.value))).toString(),
		});
		const response = await fetch(`/api/coverage?${params}`, { signal: coverageController.signal });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || "3D coverage is unavailable.");
		coverageTiles = data.tiles || [];
    coverageState = "ready";
    renderCoverage();
    els.coverageToggle.disabled = false;
		const available = data.availableCount || 0;
		const unknown = data.unknownCount || 0;
		els.coverageToggle.classList.toggle("is-unknown", available === 0 && unknown > 0);
		els.coverageCount.textContent = available
			? `${available.toLocaleString()} confirmed · ${unknown.toLocaleString()} unindexed`
			: unknown
				? `${unknown.toLocaleString()} tiles not indexed by Apple`
				: "No 3D tiles in view";
		const center = map.getCenter();
		const centerTile = coverageTileAt(center.lat, center.lng);
		els.areaPrompt.textContent = centerTile?.available
			? "Green tiles have confirmed 3D mesh data."
			: centerTile && !centerTile.known
				? "Apple does not publish a coverage index here. Exports will verify the tiles directly."
				: "No confirmed 3D tile at the map center. Try a nearby green area.";
		if (!selectionBounds) {
			els.estimate.textContent = available
				? "Choose an area inside a green tile."
				: unknown
					? "Mark an area. Unindexed tiles will be verified during export."
					: "No 3D tiles are indexed in this view.";
		}
    else updateSelection();
  } catch (error) {
		if (error.name === "AbortError") return;
    coverageState = "error";
		els.coverageToggle.classList.remove("is-unknown");
    els.coverageToggle.classList.add("has-error");
    const message = error.message || "Coverage check unavailable";
    els.coverageCount.textContent = message;
    els.areaPrompt.textContent = "Coverage could not be pre-checked. The export will verify every selected tile directly.";
		if (!selectionBounds) els.estimate.textContent = "Mark an area. Orbis will verify its 3D data during export.";
    if (selectionBounds) updateSelection();
  }
}

function renderCoverage() {
  coverageLayer.clearLayers();
	coverageTiles.filter((tile) => tile.available || !tile.known).forEach((tile) => {
		L.rectangle([[tile.south, tile.west], [tile.north, tile.east]], {
      renderer: coverageRenderer,
      interactive: false,
      stroke: true,
			color: tile.available ? "#2f7a2a" : "#e07d16",
			weight: 1,
			opacity: tile.available ? 0.72 : 0.42,
			dashArray: tile.available ? undefined : "5 5",
			fillColor: tile.available ? "#58b03a" : "#f9a13a",
			fillOpacity: tile.available ? 0.18 : 0,
    }).addTo(coverageLayer);
  });
}

function coverageTileAt(lat, lon) {
	return coverageTiles.find((tile) => lat >= tile.south && lat <= tile.north && lon >= tile.west && lon <= tile.east) || null;
}

function selectionCoverage(req) {
	if (coverageState === "error") return { type: "unknown", label: "Verify on export" };
	if (coverageState !== "ready") return { type: "loading", label: "Checking 3D" };
  const centerLat = (req.north + req.south) / 2;
  const centerLon = (req.west + req.east) / 2;
	const centerTile = coverageTileAt(centerLat, centerLon);
	if (!centerTile) return { type: "loading", label: "Checking 3D" };
	if (!centerTile.known) return { type: "unknown", label: "Not indexed" };
	if (!centerTile.available) return { type: "none", label: "No 3D data" };

  const samples = [
    [req.north, req.west], [req.north, centerLon], [req.north, req.east],
    [centerLat, req.west], [centerLat, centerLon], [centerLat, req.east],
    [req.south, req.west], [req.south, centerLon], [req.south, req.east],
  ];
	const fullyCovered = samples.every(([lat, lon]) => coverageTileAt(lat, lon)?.available);
  return fullyCovered
    ? { type: "full", label: "3D available" }
    : { type: "partial", label: "Partly covered" };
}

function setExporting(exporting) {
  els.exportButton.disabled = exporting;
  els.exportButton.querySelector("span").textContent = exporting ? "Exporting…" : exportLabel();
}

function estimateRequests(req) {
  const nw = latLonToTileTMS(req.zoom, req.north, req.west);
  const se = latLonToTileTMS(req.zoom, req.south, req.east);
  const xMin = Math.min(nw.x, se.x);
  const xMax = Math.max(nw.x, se.x);
  const yMin = Math.min(nw.y, se.y);
  const yMax = Math.max(nw.y, se.y);
  const tiles = (xMax - xMin + 1) * (yMax - yMin + 1);
  return { xMin, xMax, yMin, yMax, tiles, requests: tiles * req.tryH };
}

function latLonToTileTMS(zoom, lat, lon) {
  const n = 2 ** zoom;
  const latRad = (lat / 180) * Math.PI;
  return {
    x: Math.floor(n * ((lon + 180) / 360)),
    y: Math.floor((Math.log(Math.tan(latRad * 0.5 + Math.PI / 4)) / (2 * Math.PI) + 0.5) * n),
  };
}

function selectionSize(req) {
  const midLat = (req.north + req.south) / 2;
  return {
    width: distanceMeters(midLat, req.west, midLat, req.east),
    height: distanceMeters(req.north, req.west, req.south, req.west),
  };
}

function distanceMeters(lat1, lon1, lat2, lon2) {
  const toRad = (value) => (value * Math.PI) / 180;
  const dLat = toRad(lat2 - lat1);
  const dLon = toRad(lon2 - lon1);
  const a = Math.sin(dLat / 2) ** 2
    + Math.cos(toRad(lat1)) * Math.cos(toRad(lat2)) * Math.sin(dLon / 2) ** 2;
  return 6371000 * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
}

function formatDistance(meters) {
  if (meters >= 1000) return `${(meters / 1000).toFixed(meters >= 10000 ? 0 : 1)} km`;
  return `${Math.max(1, Math.round(meters))} m`;
}

function today() {
  return new Intl.DateTimeFormat("en-CA", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date());
}

async function pollJob(id) {
  window.clearTimeout(pollTimer);
  try {
    const response = await fetch(`/api/jobs/${id}`);
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || "The export status could not be loaded.");

    const exported = data.exported || 0;
    const processed = data.processed || 0;
    const total = data.total || 0;
    updateJobProgress(processed, total);
    els.jobLog.textContent = (data.logs?.length ? data.logs : ["The exporter is getting ready."]).join("\n");
    els.jobLog.scrollTop = els.jobLog.scrollHeight;

    if (data.state === "running") {
      els.jobState.textContent = processed ? "Loading map tiles…" : "Preparing the tile scan…";
      const percent = total ? Math.min(100, Math.round((processed / total) * 100)) : 0;
      els.jobCount.textContent = total
        ? `${processed.toLocaleString()} / ${total.toLocaleString()} tile requests · ${percent}% · ${exported.toLocaleString()} meshes written`
        : "Calculating the scan…";
      pollTimer = window.setTimeout(() => pollJob(id), 1000);
      return;
    }

    setExporting(false);
    updateExportButton();
    if (data.state === "error") {
      showJobError(data.error || "The export stopped before it finished.");
      return;
    }

    els.jobState.textContent = "Export ready";
    els.jobCount.textContent = `${exported} ${exported === 1 ? "mesh" : "meshes"} saved`;
    els.progressBar.classList.add("done");
    els.jobIcon.className = "job-icon is-done";
    els.jobIcon.innerHTML = '<i data-lucide="check" aria-hidden="true"></i>';
    if (data.output) {
      els.outputPath.hidden = false;
      els.outputPath.querySelector("span").textContent = data.output;
      els.outputPath.removeAttribute("href");
    }
		if (data.world) {
			els.worldPath.hidden = false;
			els.worldPath.querySelector("span").textContent = data.world;
			els.worldPath.removeAttribute("href");
			els.worldHint.hidden = false;
		}
		// The viewer needs the OBJ pair, so a Minecraft-only export has nothing to show.
		if (data.output && els.objInput.checked) {
			els.viewerLink.hidden = false;
			els.viewerLink.href = `/viewer.html?job=${encodeURIComponent(id)}`;
		}
    els.liveRegion.textContent = `Export ready. ${exported} meshes saved.`;
    refreshIcons();
  } catch (error) {
    showJobError(error.message);
  }
}

function showJobError(message) {
  window.clearTimeout(pollTimer);
  setExporting(false);
  updateExportButton();
  els.jobCard.hidden = false;
  els.jobState.textContent = "Export stopped";
  els.jobCount.textContent = message;
  els.jobLog.textContent += `\n${message}`;
  els.progressBar.classList.add("error");
  els.jobIcon.className = "job-icon is-error";
  els.jobIcon.innerHTML = '<i data-lucide="circle-alert" aria-hidden="true"></i>';
  els.liveRegion.textContent = `Export stopped. ${message}`;
  refreshIcons();
}

function updateJobProgress(processed, total) {
  const percent = total ? Math.min(100, Math.max(0, (processed / total) * 100)) : 0;
  const fill = els.progressBar.querySelector("span");
  fill.style.width = `${percent}%`;
  els.progressBar.setAttribute("aria-valuenow", String(Math.round(percent)));
  els.progressBar.setAttribute("aria-valuetext", total
    ? `${processed.toLocaleString()} of ${total.toLocaleString()} tile requests checked`
    : "Preparing export");
}

async function loadSettings() {
  try {
    const response = await fetch("/api/settings");
    if (!response.ok) throw new Error("Orbis could not check its local setup.");
    const settings = await response.json();
    if (Number.isInteger(settings.maxRequests) && settings.maxRequests > 0) {
      exportRequestLimit = settings.maxRequests;
      updateSelection();
    }
    if (settings.configReady) {
      els.setupGate.hidden = true;
      els.setupGate.setAttribute("aria-busy", "false");
      return true;
    }
    showSetupImport(settings.platform);
    return false;
  } catch (_error) {
    showSetupImport();
    els.setupStatus.textContent = "Orbis konnte die lokale Einrichtung nicht prüfen. Wähle die config.json manuell aus.";
    els.setupStatus.classList.add("is-error");
    return false;
  }
}

function showSetupImport(platform = "") {
  els.setupChecking.hidden = true;
  els.setupImport.hidden = false;
  els.setupGate.hidden = false;
  els.setupGate.setAttribute("aria-busy", "false");
  if (platform === "darwin") {
    els.setupStatus.textContent = "Die automatische macOS-Erkennung war nicht möglich. Die Datei wird nur lokal gespeichert.";
  }
  refreshIcons();
  window.setTimeout(() => els.setupDropzone.focus(), 0);
}

async function importSetupFile(file) {
  if (file.size > 64 * 1024) {
    showSetupError("Diese Datei ist zu groß für eine Orbis-Konfiguration.");
    return;
  }

  let raw;
  try {
    raw = await file.text();
    const parsed = JSON.parse(raw);
    if (!parsed || typeof parsed !== "object" || typeof parsed.tokenP1 !== "string") {
      throw new Error("missing token");
    }
  } catch (_error) {
    showSetupError("Das ist keine gültige config.json für Orbis.");
    return;
  }

  els.setupDropzone.disabled = true;
  els.setupStatus.className = "setup-status is-working";
  els.setupStatus.textContent = "Konfiguration wird lokal geprüft…";
  try {
    const response = await fetch("/api/setup/config", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: raw,
    });
    const result = await response.json();
    if (!response.ok || !result.ready) throw new Error(result.error || "Die Konfiguration funktioniert nicht.");
    finishSetup();
  } catch (error) {
    showSetupError(error.message);
  } finally {
    els.setupDropzone.disabled = false;
    els.setupFileInput.value = "";
  }
}

els.setupScanForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const path = els.setupScanPath.value.trim();
  if (!path) return;

  els.setupStatus.className = "setup-status is-working";
  els.setupStatus.textContent = "Datei wird lokal nach dem Clientwert durchsucht…";
  try {
    const response = await fetch("/api/setup/scan", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path }),
    });
    const result = await response.json();
    if (!response.ok || !result.ready) throw new Error(result.error || "Die Datei konnte nicht verwendet werden.");
    finishSetup();
  } catch (error) {
    showSetupError(error.message);
  }
});

function finishSetup() {
  els.setupStatus.className = "setup-status";
  els.setupStatus.textContent = "Fertig. Orbis ist startklar.";
  els.liveRegion.textContent = "Orbis ist startklar.";
  window.setTimeout(() => {
    els.setupGate.hidden = true;
    els.searchInput.focus();
    loadCoverage();
  }, 450);
}

function showSetupError(message) {
  els.setupStatus.className = "setup-status is-error";
  els.setupStatus.textContent = message;
  els.liveRegion.textContent = message;
}

function refreshIcons() {
  if (window.lucide) window.lucide.createIcons({ attrs: { "stroke-width": 1.8 } });
}

refreshIcons();
updateFormats();
loadSettings().then((ready) => {
  if (ready) loadCoverage();
});
