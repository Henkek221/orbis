import * as THREE from "three";
import { MTLLoader } from "three/addons/loaders/MTLLoader.js";
import { OBJLoader } from "three/addons/loaders/OBJLoader.js";
import { GLTFLoader } from "three/addons/loaders/GLTFLoader.js";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";

const els = {
  viewport: document.getElementById("viewport"),
  scanTitle: document.getElementById("scanTitle"),
  statusDot: document.getElementById("statusDot"),
  viewerStatus: document.getElementById("viewerStatus"),
  viewerStats: document.getElementById("viewerStats"),
  spawnButton: document.getElementById("spawnButton"),
  driveButton: document.getElementById("driveButton"),
  cameraButton: document.getElementById("cameraButton"),
  overviewButton: document.getElementById("overviewButton"),
  openModelButton: document.getElementById("openModelButton"),
  axisSelect: document.getElementById("axisSelect"),
  smoothToggle: document.getElementById("smoothToggle"),
  wireframeToggle: document.getElementById("wireframeToggle"),
  timeInput: document.getElementById("timeInput"),
  timeValue: document.getElementById("timeValue"),
  headlightToggle: document.getElementById("headlightToggle"),
  spawnHelp: document.getElementById("spawnHelp"),
  driveHelp: document.getElementById("driveHelp"),
  speedValue: document.getElementById("speedValue"),
  loadingPanel: document.getElementById("loadingPanel"),
  loadingCopy: document.getElementById("loadingCopy"),
  loadingFill: document.getElementById("loadingFill"),
  dropZone: document.getElementById("dropZone"),
  dropTitle: document.getElementById("dropTitle"),
  dropCopy: document.getElementById("dropCopy"),
  dropButton: document.getElementById("dropButton"),
  dropDismiss: document.getElementById("dropDismiss"),
  modelFiles: document.getElementById("modelFiles"),
};

const renderer = new THREE.WebGLRenderer({ antialias: true, powerPreference: "high-performance" });
renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
renderer.setSize(window.innerWidth, window.innerHeight);
renderer.outputColorSpace = THREE.SRGBColorSpace;
renderer.toneMapping = THREE.AgXToneMapping;
renderer.toneMappingExposure = 1.08;
renderer.shadowMap.enabled = true;
renderer.shadowMap.type = THREE.PCFSoftShadowMap;
els.viewport.append(renderer.domElement);

const scene = new THREE.Scene();
scene.background = new THREE.Color(0xb9cfd2);
scene.fog = new THREE.Fog(0xb9cfd2, 500, 3200);

const camera = new THREE.PerspectiveCamera(60, window.innerWidth / window.innerHeight, 0.08, 10000);
camera.position.set(40, 35, 40);
const orbit = new OrbitControls(camera, renderer.domElement);
orbit.enableDamping = true;
orbit.dampingFactor = 0.075;
orbit.screenSpacePanning = true;

const hemisphere = new THREE.HemisphereLight(0xf1f7f8, 0x66705f, 2.1);
scene.add(hemisphere);
const sun = new THREE.DirectionalLight(0xfff3dc, 2.5);
sun.position.set(-240, 480, 160);
sun.castShadow = true;
scene.add(sun);

const raycaster = new THREE.Raycaster();
const pointer = new THREE.Vector2();
const clock = new THREE.Clock();
const up = new THREE.Vector3(0, 1, 0);
const ringNormal = new THREE.Vector3(0, 0, 1);
const keys = { forward: false, back: false, left: false, right: false, boost: false };

let model = null;
let modelBounds = null;
let overviewPosition = null;
let overviewTarget = null;
let carRoot = null;
let spawn = null;
let spawnMode = false;
let driving = false;
let speed = 0;
let heading = 0;
let cameraMode = "chase";
let lastSurfaceY = null;
let timeSky = null;
let timeOfDay = 14;
let modelSourceUp = "z";
let dragDepth = 0;
let localObjectURLs = [];
const headlights = [];

const spawnMarker = new THREE.Mesh(
  new THREE.RingGeometry(1.7, 2.15, 40),
  new THREE.MeshBasicMaterial({ color: 0xe15b3d, side: THREE.DoubleSide, transparent: true, opacity: 0.9, depthTest: false }),
);
spawnMarker.renderOrder = 10;
spawnMarker.visible = false;
scene.add(spawnMarker);

els.spawnButton.addEventListener("click", () => setSpawnMode(!spawnMode));
els.driveButton.addEventListener("click", () => {
  if (driving) stopDriving();
  else startDriving();
});
els.cameraButton.addEventListener("click", toggleCamera);
els.overviewButton.addEventListener("click", resetOverview);
els.openModelButton.addEventListener("click", () => showDropOverlay(false));
els.dropButton.addEventListener("click", () => els.modelFiles.click());
els.dropDismiss.addEventListener("click", hideDropOverlay);
els.modelFiles.addEventListener("change", async () => {
  const entries = [...els.modelFiles.files].map((file) => ({ file, path: file.webkitRelativePath || file.name }));
  els.modelFiles.value = "";
  await loadLocalModel(entries);
});
els.axisSelect.addEventListener("change", () => {
  if (!model) return;
  orientModel();
  clearVehiclePlacement();
  prepareOverview();
  setTimeOfDay(timeOfDay);
  els.viewerStatus.textContent = "Model orientation updated";
});
els.smoothToggle.addEventListener("change", applySurfaceSettings);
els.wireframeToggle.addEventListener("change", applySurfaceSettings);
els.timeInput.addEventListener("input", () => setTimeOfDay(Number(els.timeInput.value)));
els.headlightToggle.addEventListener("change", updateHeadlights);

renderer.domElement.addEventListener("pointermove", previewSpawn);
renderer.domElement.addEventListener("pointerleave", () => {
  if (spawnMode && !spawn) spawnMarker.visible = false;
});
renderer.domElement.addEventListener("click", placeSpawn);
renderer.domElement.addEventListener("dblclick", moveOverviewTarget);
window.addEventListener("keydown", (event) => setKey(event, true));
window.addEventListener("keyup", (event) => setKey(event, false));
window.addEventListener("resize", resize);
window.addEventListener("dragenter", handleDragEnter);
window.addEventListener("dragover", handleDragOver);
window.addEventListener("dragleave", handleDragLeave);
window.addEventListener("drop", handleDrop);

async function loadExport() {
  const jobID = new URLSearchParams(window.location.search).get("job");
  if (!jobID) return showDropWelcome();

  try {
    const response = await fetch(`/api/jobs/${encodeURIComponent(jobID)}`);
    const job = await response.json();
    if (!response.ok) throw new Error(job.error || "The export could not be found.");
    if (job.state !== "done") throw new Error("This export is not ready yet.");
    beginModelLoad();
    els.scanTitle.textContent = humanName(job.name || jobID);

    const base = `/api/jobs/${encodeURIComponent(jobID)}/files/`;
    const manager = new THREE.LoadingManager();
    manager.onProgress = (_url, loaded, total) => {
      const percent = total ? Math.round((loaded / total) * 100) : 0;
      els.loadingCopy.textContent = `Loading scan assets ${loaded} of ${total}.`;
      els.loadingFill.style.width = `${Math.min(percent, 90)}%`;
    };

    const mtlLoader = new MTLLoader(manager);
    mtlLoader.setPath(base);
    mtlLoader.setResourcePath(base);
    mtlLoader.setMaterialOptions({ side: THREE.DoubleSide });
    const materials = await mtlLoader.loadAsync("exp_model.mtl");
    materials.preload();

    const objLoader = new OBJLoader(manager);
    objLoader.setPath(base);
    objLoader.setMaterials(materials);
    model = await objLoader.loadAsync("exp_model.obj");
    modelSourceUp = "z";
    els.axisSelect.value = "z";
    els.axisSelect.disabled = false;
    orientModel();
    model.updateMatrixWorld(true);
    scene.add(model);

    els.loadingCopy.textContent = "Loading the vehicle.";
    els.loadingFill.style.width = "92%";
    if (!carRoot) await loadVehicle();
    applySurfaceSettings();
    prepareOverview();
    setupTimeOfDay();

    finishModelLoad();
  } catch (error) {
    showError(error.message);
  }
}

function showDropWelcome() {
  els.loadingPanel.hidden = true;
  els.scanTitle.textContent = "Open a local OBJ";
  els.statusDot.className = "status-dot is-loading";
  els.viewerStatus.textContent = "Drop a model to begin";
  els.viewerStats.textContent = "OBJ · optional MTL and textures";
  showDropOverlay(false);
}

function showDropOverlay(active) {
  if (driving) stopDriving();
  setSpawnMode(false);
  els.dropZone.hidden = false;
  els.dropZone.classList.toggle("is-active", active);
  els.dropDismiss.hidden = !model;
  els.dropTitle.textContent = active ? "Release to open the model" : "Drop an OBJ here";
  els.dropCopy.textContent = "Add the OBJ with its MTL and texture files, or drop the whole export folder.";
}

function hideDropOverlay() {
  if (!model) return;
  dragDepth = 0;
  els.dropZone.classList.remove("is-active");
  els.dropZone.hidden = true;
}

function hasDraggedFiles(event) {
  return [...(event.dataTransfer?.types || [])].includes("Files");
}

function handleDragEnter(event) {
  if (!hasDraggedFiles(event)) return;
  event.preventDefault();
  dragDepth += 1;
  showDropOverlay(true);
}

function handleDragOver(event) {
  if (!hasDraggedFiles(event)) return;
  event.preventDefault();
  event.dataTransfer.dropEffect = "copy";
}

function handleDragLeave(event) {
  if (!hasDraggedFiles(event)) return;
  event.preventDefault();
  dragDepth = Math.max(0, dragDepth - 1);
  if (dragDepth > 0) return;
  els.dropZone.classList.remove("is-active");
  if (model) els.dropZone.hidden = true;
  else showDropOverlay(false);
}

async function handleDrop(event) {
  if (!hasDraggedFiles(event)) return;
  event.preventDefault();
  dragDepth = 0;
  els.dropZone.classList.remove("is-active");
  const entries = await droppedFileEntries(event.dataTransfer);
  await loadLocalModel(entries);
}

async function droppedFileEntries(dataTransfer) {
  const itemEntries = [...(dataTransfer?.items || [])]
    .map((item) => item.webkitGetAsEntry?.())
    .filter(Boolean);
  if (!itemEntries.length) {
    return [...(dataTransfer?.files || [])].map((file) => ({ file, path: file.name }));
  }
  const nested = await Promise.all(itemEntries.map((entry) => walkDroppedEntry(entry)));
  return nested.flat();
}

async function walkDroppedEntry(entry) {
  if (entry.isFile) {
    const file = await new Promise((resolve, reject) => entry.file(resolve, reject));
    return [{ file, path: entry.fullPath?.replace(/^\//, "") || file.name }];
  }
  if (!entry.isDirectory) return [];
  const reader = entry.createReader();
  const children = [];
  while (true) {
    const batch = await new Promise((resolve, reject) => reader.readEntries(resolve, reject));
    if (!batch.length) break;
    children.push(...batch);
  }
  const nested = await Promise.all(children.map((child) => walkDroppedEntry(child)));
  return nested.flat();
}

async function loadLocalModel(entries) {
  const objEntries = entries.filter((entry) => extensionOf(entry.path) === ".obj");
  if (objEntries.length !== 1) {
    showImportError(objEntries.length ? "Drop one OBJ model at a time." : "No OBJ file was found. Add the model file as well as its supporting files.", true);
    return;
  }

  try {
    beginModelLoad();
    const objEntry = objEntries[0];
    els.scanTitle.textContent = humanName(fileStem(objEntry.file.name));
    els.loadingCopy.textContent = `Reading ${objEntry.file.name}.`;
    els.loadingFill.style.width = "12%";
    await nextPaint();

    const objText = await objEntry.file.text();
    els.loadingCopy.textContent = "Matching materials and textures.";
    els.loadingFill.style.width = "32%";

    const fileIndex = indexLocalFiles(entries);
    const objectURLByFile = new Map();
    let missingAssets = 0;
    const manager = new THREE.LoadingManager();
    manager.setURLModifier((url) => {
      const key = normalizeAssetPath(url.replace(/^local:\/\//i, ""));
      const file = fileIndex.get(key) || fileIndex.get(baseName(key));
      if (!file) return url;
      if (!objectURLByFile.has(file)) {
        const objectURL = URL.createObjectURL(file);
        objectURLByFile.set(file, objectURL);
        localObjectURLs.push(objectURL);
      }
      return objectURLByFile.get(file);
    });
    manager.onError = () => {
      missingAssets += 1;
      els.viewerStatus.textContent = "Model ready · some textures are missing";
    };

    let materials = null;
    const mtlEntry = matchingMaterialEntry(objText, entries, fileIndex);
    if (mtlEntry) {
      const mtlText = await mtlEntry.file.text();
      const mtlLoader = new MTLLoader(manager);
      mtlLoader.setMaterialOptions({ side: THREE.DoubleSide });
      materials = mtlLoader.parse(mtlText, "local://");
      materials.preload();
    }

    els.loadingCopy.textContent = "Parsing model geometry.";
    els.loadingFill.style.width = "58%";
    await nextPaint();
    const objLoader = new OBJLoader(manager);
    if (materials) objLoader.setMaterials(materials);
    model = objLoader.parse(objText);
    if (!model.children.length) throw new Error("The OBJ contains no renderable geometry.");

    modelSourceUp = "auto";
    els.axisSelect.value = "auto";
    els.axisSelect.disabled = false;
    orientModel();
    model.updateMatrixWorld(true);
    scene.add(model);

    els.loadingCopy.textContent = "Preparing the vehicle and surface tools.";
    els.loadingFill.style.width = "86%";
    if (!carRoot) await loadVehicle();
    applySurfaceSettings();
    prepareOverview();
    setupTimeOfDay();
    finishModelLoad();
    if (!materials) {
      els.viewerStatus.textContent = "Model ready · no MTL supplied";
    } else if (missingAssets) {
      els.viewerStatus.textContent = "Model ready · some textures are missing";
    }
  } catch (error) {
    showImportError(error.message || "The model could not be opened.");
  }
}

function beginModelLoad() {
  clearCurrentModel();
  els.dropZone.hidden = true;
  els.dropZone.classList.remove("is-active");
  els.loadingPanel.hidden = false;
  els.loadingFill.parentElement.classList.remove("is-error");
  els.loadingFill.style.width = "4%";
  els.loadingCopy.textContent = "Preparing model files.";
  els.statusDot.className = "status-dot is-loading";
  els.viewerStatus.textContent = "Loading geometry";
  els.viewerStats.textContent = "Large models can take a moment.";
  els.spawnButton.disabled = true;
  els.driveButton.disabled = true;
  els.cameraButton.disabled = true;
  els.overviewButton.disabled = true;
  els.axisSelect.disabled = true;
}

function finishModelLoad() {
  els.loadingFill.style.width = "100%";
  els.loadingPanel.hidden = true;
  els.statusDot.className = "status-dot";
  els.viewerStatus.textContent = "Ready to place the vehicle";
  els.spawnButton.disabled = false;
  els.overviewButton.disabled = false;
  els.axisSelect.disabled = false;
  els.dropDismiss.hidden = false;
  setSpawnMode(true);
}

function showImportError(message, preserveCurrentModel = false) {
  if (!preserveCurrentModel) clearCurrentModel();
  els.loadingPanel.hidden = true;
  els.statusDot.className = "status-dot is-error";
  els.viewerStatus.textContent = "Model could not be opened";
  els.viewerStats.textContent = message;
  els.dropTitle.textContent = "Could not open this model";
  els.dropCopy.textContent = message;
  els.dropZone.classList.remove("is-active");
  els.dropZone.hidden = false;
  els.dropDismiss.hidden = !model;
}

function clearCurrentModel() {
  clearVehiclePlacement();
  if (model) {
    scene.remove(model);
    disposeObject(model);
  }
  model = null;
  modelBounds = null;
  overviewPosition = null;
  overviewTarget = null;
  if (timeSky) {
    scene.remove(timeSky);
    timeSky.geometry.dispose();
    timeSky.material.dispose();
    timeSky = null;
  }
  localObjectURLs.forEach((url) => URL.revokeObjectURL(url));
  localObjectURLs = [];
}

function clearVehiclePlacement() {
  if (driving) stopDriving();
  spawnMode = false;
  spawn = null;
  speed = 0;
  lastSurfaceY = null;
  spawnMarker.visible = false;
  renderer.domElement.classList.remove("is-placing");
  els.spawnButton.classList.remove("is-active");
  els.spawnHelp.hidden = true;
  els.driveHelp.hidden = true;
  els.spawnButton.textContent = "Set vehicle spawn";
  els.driveButton.textContent = "Drive from this spawn";
  els.driveButton.classList.remove("is-driving");
  els.driveButton.disabled = true;
  els.cameraButton.disabled = true;
  if (carRoot) carRoot.visible = false;
}

function disposeObject(root) {
  root.traverse((child) => {
    if (!child.isMesh) return;
    child.geometry?.dispose();
    const materials = Array.isArray(child.material) ? child.material : [child.material];
    materials.forEach((material) => {
      material?.map?.dispose();
      material?.dispose();
    });
  });
}

function orientModel() {
  if (!model) return;
  model.rotation.set(0, 0, 0);
  model.updateMatrixWorld(true);
  let axis = els.axisSelect.value;
  if (axis === "auto") axis = detectModelUpAxis();
  if (axis === "z") model.rotation.x = -Math.PI / 2;
  model.updateMatrixWorld(true);
}

function detectModelUpAxis() {
  if (modelSourceUp !== "auto") return modelSourceUp;
  const size = new THREE.Box3().setFromObject(model).getSize(new THREE.Vector3());
  if (size.z < size.y * 0.72) return "z";
  if (size.y < size.z * 0.72) return "y";
  return "y";
}

function indexLocalFiles(entries) {
  const index = new Map();
  entries.forEach(({ file, path }) => {
    const normalized = normalizeAssetPath(path || file.name);
    index.set(normalized, file);
    if (!index.has(baseName(normalized))) index.set(baseName(normalized), file);
  });
  return index;
}

function matchingMaterialEntry(objText, entries, fileIndex) {
  const match = objText.match(/^\s*mtllib\s+(.+?)\s*$/im);
  if (match) {
    const requested = normalizeAssetPath(match[1].replace(/^['"]|['"]$/g, ""));
    const requestedFile = fileIndex.get(requested) || fileIndex.get(baseName(requested));
    if (requestedFile) return entries.find((entry) => entry.file === requestedFile) || null;
  }
  const materials = entries.filter((entry) => extensionOf(entry.path) === ".mtl");
  return materials.length === 1 ? materials[0] : null;
}

function normalizeAssetPath(value) {
  let decoded = value;
  try { decoded = decodeURIComponent(value); } catch (_) { /* keep original path */ }
  return decoded.replace(/\\/g, "/").replace(/^\.\//, "").replace(/^\//, "").toLocaleLowerCase();
}

function baseName(value) {
  return value.split("/").pop() || value;
}

function extensionOf(value) {
  const name = baseName(normalizeAssetPath(value));
  const dot = name.lastIndexOf(".");
  return dot >= 0 ? name.slice(dot) : "";
}

function fileStem(value) {
  const dot = value.lastIndexOf(".");
  return dot > 0 ? value.slice(0, dot) : value;
}

function nextPaint() {
  return new Promise((resolve) => requestAnimationFrame(() => resolve()));
}

async function loadVehicle() {
  const gltf = await new GLTFLoader().loadAsync("/assets/toy-car.glb");
  const visual = new THREE.Group();
  ["ToyCar", "Glass"].forEach((name) => {
    const part = gltf.scene.getObjectByName(name);
    if (part) visual.add(part.clone());
  });
  if (!visual.children.length) throw new Error("The vehicle model could not be prepared.");

  visual.updateMatrixWorld(true);
  const initialBounds = new THREE.Box3().setFromObject(visual);
  const initialSize = initialBounds.getSize(new THREE.Vector3());
  const roadLength = Math.max(initialSize.x, initialSize.z);
  const scale = 4.35 / Math.max(roadLength, 0.001);
  visual.scale.setScalar(scale);
  visual.updateMatrixWorld(true);

  const bounds = new THREE.Box3().setFromObject(visual);
  const center = bounds.getCenter(new THREE.Vector3());
  visual.position.x -= center.x;
  visual.position.y -= bounds.min.y;
  visual.position.z -= center.z;
  if (initialSize.x > initialSize.z) visual.rotation.y = Math.PI / 2;

  visual.traverse((child) => {
    if (!child.isMesh) return;
    child.castShadow = true;
    child.receiveShadow = true;
  });

  carRoot = new THREE.Group();
  carRoot.visible = false;
  carRoot.add(visual);
  scene.add(carRoot);
  addHeadlights();
}

function applySurfaceSettings() {
  if (!model) return;
  let triangles = 0;
  let meshes = 0;
  const anisotropy = renderer.capabilities.getMaxAnisotropy();
  model.traverse((child) => {
    if (!child.isMesh) return;
    meshes += 1;
    const geometry = child.geometry;
    if (els.smoothToggle.checked) geometry.computeVertexNormals();
    const count = geometry.index ? geometry.index.count : geometry.attributes.position?.count || 0;
    triangles += Math.floor(count / 3);
    const materials = Array.isArray(child.material) ? child.material : [child.material];
    materials.forEach((material) => {
      material.flatShading = !els.smoothToggle.checked;
      material.wireframe = els.wireframeToggle.checked;
      material.side = THREE.DoubleSide;
      if (material.map) {
        material.map.colorSpace = THREE.SRGBColorSpace;
        material.map.anisotropy = anisotropy;
      }
      material.needsUpdate = true;
    });
  });
  els.viewerStats.textContent = `${triangles.toLocaleString()} triangles · ${meshes.toLocaleString()} mesh sections`;
}

function prepareOverview() {
  modelBounds = new THREE.Box3().setFromObject(model);
  const size = modelBounds.getSize(new THREE.Vector3());
  const center = modelBounds.getCenter(new THREE.Vector3());
  const radius = Math.max(size.x, size.y, size.z, 10);
  overviewTarget = center.clone();
  overviewPosition = center.clone().add(new THREE.Vector3(radius * 0.72, radius * 0.58, radius * 0.72));
  camera.near = Math.max(0.05, radius / 20000);
  camera.far = Math.max(4000, radius * 12);
  camera.updateProjectionMatrix();
  resetOverview();
}

function setupTimeOfDay() {
  createTimeSky();
  if (!sun.target.parent) scene.add(sun.target);
  setTimeOfDay(timeOfDay);
  updateHeadlights();
}

function createTimeSky() {
  const sphere = new THREE.Sphere();
  modelBounds.getBoundingSphere(sphere);
  const geometry = new THREE.SphereGeometry(Math.max(1800, sphere.radius * 8), 48, 24);
  const material = new THREE.ShaderMaterial({
    side: THREE.BackSide,
    depthWrite: false,
    uniforms: {
      topColor: { value: new THREE.Color(0x74b6e4) },
      horizonColor: { value: new THREE.Color(0xd9e9ee) },
      groundColor: { value: new THREE.Color(0x9eb2bb) },
      sunDirection: { value: new THREE.Vector3(0, 1, 0) },
      sunColor: { value: new THREE.Color(0xfff1d2) },
      sunAmount: { value: 1 },
    },
    vertexShader: `
      varying vec3 vDirection;
      void main() {
        vDirection = normalize(position);
        gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
      }
    `,
    fragmentShader: `
      uniform vec3 topColor;
      uniform vec3 horizonColor;
      uniform vec3 groundColor;
      uniform vec3 sunDirection;
      uniform vec3 sunColor;
      uniform float sunAmount;
      varying vec3 vDirection;

      void main() {
        vec3 direction = normalize(vDirection);
        float upper = smoothstep(-0.04, 0.72, direction.y);
        float lower = smoothstep(-0.55, 0.02, direction.y);
        vec3 color = mix(groundColor, horizonColor, lower);
        color = mix(color, topColor, upper);
        float sunDot = max(dot(direction, normalize(sunDirection)), 0.0);
        float glow = pow(sunDot, 96.0) * 0.18;
        float disk = smoothstep(0.99988, 0.99997, sunDot);
        color += sunColor * (glow + disk * 2.4) * sunAmount;
        gl_FragColor = vec4(color, 1.0);
      }
    `,
  });
  timeSky = new THREE.Mesh(geometry, material);
  timeSky.frustumCulled = false;
  timeSky.renderOrder = -100;
  scene.add(timeSky);
}

function setTimeOfDay(hour) {
  timeOfDay = ((hour % 24) + 24) % 24;
  els.timeInput.value = String(hour);
  els.timeValue.textContent = formatTime(timeOfDay);

  const phase = ((timeOfDay - 6) / 12) * Math.PI;
  const elevation = Math.sin(phase);
  const daylight = smoothstep(-0.2, 0.1, elevation);
  const horizonLight = Math.exp(-Math.pow(elevation / 0.24, 2));
  const sunsetSide = timeOfDay >= 12 ? 1 : 0.72;
  const golden = horizonLight * sunsetSide;

  const nightTop = new THREE.Color(0x050b18);
  const nightHorizon = new THREE.Color(0x111b2d);
  const nightGround = new THREE.Color(0x080d16);
  const dayTop = new THREE.Color(0x70b7e8);
  const dayHorizon = new THREE.Color(0xdcecf1);
  const dayGround = new THREE.Color(0x91aab4);
  const sunsetTop = new THREE.Color(0x304d78);
  const sunsetHorizon = new THREE.Color(0xff8454);
  const sunsetGround = new THREE.Color(0x59404c);

  const top = nightTop.clone().lerp(dayTop, daylight).lerp(sunsetTop, golden * 0.62);
  const horizon = nightHorizon.clone().lerp(dayHorizon, daylight).lerp(sunsetHorizon, golden * 0.92);
  const ground = nightGround.clone().lerp(dayGround, daylight).lerp(sunsetGround, golden * 0.68);
  const sunColor = new THREE.Color(0xffe8ba).lerp(new THREE.Color(0xff7b43), golden * 0.78);

  const azimuth = ((timeOfDay - 6) / 24) * Math.PI * 2;
  const sunDirection = new THREE.Vector3(
    Math.cos(azimuth) * Math.max(0.2, Math.cos(Math.asin(THREE.MathUtils.clamp(elevation, -1, 1)))),
    elevation,
    Math.sin(azimuth) * 0.72,
  ).normalize();

  if (timeSky) {
    const uniforms = timeSky.material.uniforms;
    uniforms.topColor.value.copy(top);
    uniforms.horizonColor.value.copy(horizon);
    uniforms.groundColor.value.copy(ground);
    uniforms.sunDirection.value.copy(sunDirection);
    uniforms.sunColor.value.copy(sunColor);
    uniforms.sunAmount.value = daylight;
  }

  scene.background.copy(horizon);
  const sceneRadius = modelBounds ? modelBounds.getSize(new THREE.Vector3()).length() : 1000;
  scene.fog = new THREE.Fog(horizon, Math.max(650, sceneRadius * 0.7), Math.max(3000, sceneRadius * 6));

  hemisphere.color.copy(new THREE.Color(0x789bc0).lerp(new THREE.Color(0xf1f7f8), daylight));
  hemisphere.groundColor.copy(new THREE.Color(0x111827).lerp(new THREE.Color(0x66705f), daylight));
  hemisphere.intensity = 0.18 + daylight * 1.92 + golden * 0.46;

  sun.color.copy(sunColor);
  sun.intensity = daylight * (0.55 + Math.max(0, elevation) * 1.95);
  const center = overviewTarget || new THREE.Vector3();
  sun.position.copy(center).addScaledVector(sunDirection, Math.max(600, sceneRadius * 1.6));
  sun.target.position.copy(center);
  renderer.toneMappingExposure = 0.7 + daylight * 0.38 + golden * 0.2;
}

function formatTime(hour) {
  const totalMinutes = Math.round(hour * 60) % (24 * 60);
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}`;
}

function smoothstep(min, max, value) {
  const t = THREE.MathUtils.clamp((value - min) / (max - min), 0, 1);
  return t * t * (3 - 2 * t);
}

function addHeadlights() {
  [-0.68, 0.68].forEach((x, index) => {
    const target = new THREE.Object3D();
    target.position.set(x * 0.35, -0.45, 32);
    carRoot.add(target);
    const light = new THREE.SpotLight(0xddeeff, 430, 88, 0.46, 0.82, 2);
    light.position.set(x, 0.72, 1.62);
    light.target = target;
    light.castShadow = index === 0;
    if (light.castShadow) {
      light.shadow.mapSize.set(1024, 1024);
      light.shadow.camera.near = 0.4;
      light.shadow.camera.far = 88;
    }
    carRoot.add(light);
    headlights.push(light);

    const lamp = new THREE.Mesh(
      new THREE.SphereGeometry(0.13, 12, 8),
      new THREE.MeshBasicMaterial({ color: 0xeaf5ff, toneMapped: false }),
    );
    lamp.position.set(x, 0.7, 1.69);
    lamp.scale.set(1.5, 0.65, 0.35);
    carRoot.add(lamp);
    headlights.push(lamp);
  });
  updateHeadlights();
}

function updateHeadlights() {
  const enabled = els.headlightToggle.checked;
  headlights.forEach((light) => { light.visible = enabled; });
}

function setSpawnMode(active) {
  if (!model || driving) return;
  spawnMode = active;
  renderer.domElement.classList.toggle("is-placing", active);
  els.spawnButton.classList.toggle("is-active", active);
  els.spawnButton.textContent = active ? "Click a road in the scan" : (spawn ? "Move vehicle spawn" : "Set vehicle spawn");
  els.spawnHelp.hidden = !active;
  orbit.enabled = true;
  if (!active && spawn) showSpawnMarker(spawn);
}

function surfaceFromPointer(event) {
  if (!model) return null;
  const rect = renderer.domElement.getBoundingClientRect();
  pointer.x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
  pointer.y = -((event.clientY - rect.top) / rect.height) * 2 + 1;
  raycaster.setFromCamera(pointer, camera);
  return raycaster.intersectObject(model, true)[0] || null;
}

function previewSpawn(event) {
  if (!spawnMode) return;
  const hit = surfaceFromPointer(event);
  if (!hit) {
    spawnMarker.visible = false;
    return;
  }
  showSpawnMarker(hit);
}

function placeSpawn(event) {
  if (!spawnMode) return;
  const hit = surfaceFromPointer(event);
  if (!hit) return;
  const normal = worldNormal(hit);
  if (normal.y < 0.55) {
    els.viewerStatus.textContent = "Choose a flatter road surface";
    return;
  }

  const viewDirection = new THREE.Vector3();
  camera.getWorldDirection(viewDirection);
  viewDirection.y = 0;
  if (viewDirection.lengthSq() < 0.001) viewDirection.set(0, 0, 1);
  viewDirection.normalize();
  heading = Math.atan2(viewDirection.x, viewDirection.z);
  spawn = { point: hit.point.clone(), normal, heading };
  lastSurfaceY = hit.point.y;
  placeCar(hit.point, normal);
  carRoot.visible = true;
  showSpawnMarker(hit);
  setSpawnMode(false);
  els.driveButton.disabled = false;
  els.cameraButton.disabled = false;
  els.viewerStatus.textContent = "Vehicle spawn set";
  els.spawnButton.textContent = "Move vehicle spawn";
}

function showSpawnMarker(hit) {
  const point = hit.point || hit;
  const normal = hit.face ? worldNormal(hit) : hit.normal;
  spawnMarker.position.copy(point).addScaledVector(normal, 0.08);
  spawnMarker.quaternion.setFromUnitVectors(ringNormal, normal);
  spawnMarker.visible = true;
}

function worldNormal(hit) {
  return hit.face.normal.clone().transformDirection(hit.object.matrixWorld).normalize();
}

function surfaceAt(x, z) {
  if (!model || !modelBounds) return null;
  const height = modelBounds.max.y + Math.max(80, modelBounds.max.y - modelBounds.min.y);
  raycaster.set(new THREE.Vector3(x, height, z), new THREE.Vector3(0, -1, 0));
  raycaster.far = Math.max(500, height - modelBounds.min.y + 100);
  const hit = raycaster.intersectObject(model, true)[0];
  if (!hit) return null;
  return { point: hit.point, normal: worldNormal(hit) };
}

function placeCar(point, normal) {
  carRoot.position.copy(point).addScaledVector(normal, 0.12);
  const align = new THREE.Quaternion().setFromUnitVectors(up, normal);
  const turn = new THREE.Quaternion().setFromAxisAngle(normal, heading);
  carRoot.quaternion.copy(turn.multiply(align));
}

function startDriving() {
  if (!spawn || !carRoot) return;
  setSpawnMode(false);
  heading = spawn.heading;
  speed = 0;
  lastSurfaceY = spawn.point.y;
  placeCar(spawn.point, spawn.normal);
  carRoot.visible = true;
  spawnMarker.visible = false;
  driving = true;
  orbit.enabled = false;
  els.driveButton.textContent = "Stop driving";
  els.driveButton.classList.add("is-driving");
  els.driveHelp.hidden = false;
  els.viewerStatus.textContent = "Driving";
  updateDriveCamera(1);
}

function stopDriving() {
  driving = false;
  speed = 0;
  keys.forward = keys.back = keys.left = keys.right = keys.boost = false;
  els.driveButton.textContent = "Drive from this spawn";
  els.driveButton.classList.remove("is-driving");
  els.driveHelp.hidden = true;
  els.speedValue.textContent = "0 km/h";
  els.viewerStatus.textContent = "Vehicle parked";
  orbit.enabled = true;
  orbit.target.copy(carRoot.position).add(new THREE.Vector3(0, 1, 0));
  orbit.update();
}

function toggleCamera() {
  cameraMode = cameraMode === "chase" ? "hood" : "chase";
  els.cameraButton.textContent = cameraMode === "chase" ? "Camera: chase" : "Camera: hood";
  if (driving) updateDriveCamera(1);
}

function setKey(event, active) {
  const handled = ["KeyW", "KeyA", "KeyS", "KeyD", "ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "ShiftLeft", "ShiftRight", "KeyC", "Escape"].includes(event.code);
  if (handled && driving) event.preventDefault();
  if (event.code === "KeyW" || event.code === "ArrowUp") keys.forward = active;
  if (event.code === "KeyS" || event.code === "ArrowDown") keys.back = active;
  if (event.code === "KeyA" || event.code === "ArrowLeft") keys.left = active;
  if (event.code === "KeyD" || event.code === "ArrowRight") keys.right = active;
  if (event.code === "ShiftLeft" || event.code === "ShiftRight") keys.boost = active;
  if (active && !event.repeat && event.code === "KeyC" && driving) toggleCamera();
  if (active && event.code === "Escape" && driving) stopDriving();
  if (active && event.code === "Escape" && !els.dropZone.hidden && model) hideDropOverlay();
}

function updateVehicle(delta) {
  if (!driving || !carRoot) return;
  const maxForward = keys.boost ? 29 : 19;
  const maxReverse = -8;
  if (keys.forward) speed += 10.5 * delta;
  else if (keys.back) speed -= speed > 0.6 ? 18 * delta : 7 * delta;
  else speed *= Math.pow(0.35, delta);
  speed = THREE.MathUtils.clamp(speed, maxReverse, maxForward);
  if (Math.abs(speed) < 0.025) speed = 0;

  const steer = (keys.left ? 1 : 0) - (keys.right ? 1 : 0);
  if (steer && Math.abs(speed) > 0.08) {
    const steerStrength = THREE.MathUtils.lerp(1.25, 0.52, Math.min(Math.abs(speed) / maxForward, 1));
    heading += steer * steerStrength * delta * Math.sign(speed);
  }

  const distance = speed * delta;
  const x = carRoot.position.x + Math.sin(heading) * distance;
  const z = carRoot.position.z + Math.cos(heading) * distance;
  const surface = surfaceAt(x, z);
  const step = surface ? Math.abs(surface.point.y - lastSurfaceY) : Infinity;
  if (!surface || surface.normal.y < 0.5 || step > Math.max(1.8, Math.abs(distance) * 2.4)) {
    speed = 0;
  } else {
    placeCar(surface.point, surface.normal);
    lastSurfaceY = surface.point.y;
  }

  els.speedValue.textContent = `${Math.round(Math.abs(speed) * 3.6)} km/h`;
  updateDriveCamera(delta);
}

function updateSkyPosition() {
  if (timeSky) timeSky.position.copy(camera.position);
}

function updateDriveCamera(delta) {
  if (!carRoot) return;
  const forward = new THREE.Vector3(0, 0, 1).applyQuaternion(carRoot.quaternion).normalize();
  const carUp = new THREE.Vector3(0, 1, 0).applyQuaternion(carRoot.quaternion).normalize();
  const desired = carRoot.position.clone();
  const target = carRoot.position.clone();
  if (cameraMode === "hood") {
    desired.addScaledVector(carUp, 1.3).addScaledVector(forward, 0.5);
    target.addScaledVector(carUp, 1.15).addScaledVector(forward, 10);
  } else {
    desired.addScaledVector(carUp, 3.1).addScaledVector(forward, -7.4);
    target.addScaledVector(carUp, 1.1).addScaledVector(forward, 2.6);
  }
  const follow = delta >= 1 ? 1 : 1 - Math.exp(-7 * delta);
  camera.position.lerp(desired, follow);
  camera.up.lerp(carUp, Math.min(1, follow * 0.6)).normalize();
  camera.lookAt(target);
}

function moveOverviewTarget(event) {
  if (!model || driving || spawnMode) return;
  const hit = surfaceFromPointer(event);
  if (!hit) return;
  orbit.target.copy(hit.point);
  orbit.update();
}

function resetOverview() {
  if (!overviewPosition || !overviewTarget) return;
  if (driving) stopDriving();
  setSpawnMode(false);
  camera.up.set(0, 1, 0);
  camera.position.copy(overviewPosition);
  orbit.target.copy(overviewTarget);
  orbit.enabled = true;
  orbit.update();
}

function showError(message) {
  els.statusDot.className = "status-dot is-error";
  els.viewerStatus.textContent = "Viewer unavailable";
  els.viewerStats.textContent = message;
  els.loadingCopy.textContent = message;
  els.loadingFill.parentElement.classList.add("is-error");
}

function humanName(value) {
  return value.replaceAll("-", " ").split(/\s+/).filter(Boolean)
    .map((word) => word.charAt(0).toLocaleUpperCase() + word.slice(1)).join(" ");
}

function resize() {
  camera.aspect = window.innerWidth / window.innerHeight;
  camera.updateProjectionMatrix();
  renderer.setSize(window.innerWidth, window.innerHeight);
}

function animate() {
  requestAnimationFrame(animate);
  const delta = Math.min(clock.getDelta(), 0.05);
  updateVehicle(delta);
  updateSkyPosition();
  if (orbit.enabled) orbit.update();
  renderer.render(scene, camera);
}

loadExport();
animate();
