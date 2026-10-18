import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import puppeteer from 'puppeteer';
import { execSync } from 'node:child_process';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const targetDir = path.resolve(__dirname, '../../docs/images');
const framesDir = path.resolve(__dirname, '../../docs/images/frames');
if (!fs.existsSync(targetDir)) fs.mkdirSync(targetDir, { recursive: true });
if (!fs.existsSync(framesDir)) fs.mkdirSync(framesDir, { recursive: true });

function buildLogoSvg() {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" width="1024" height="1024">
  <defs>
    <clipPath id="squircle-clip">
      <rect x="24" y="24" width="976" height="976" rx="220" />
    </clipPath>

    <linearGradient id="edge-cyan" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#38bdf8"/>
      <stop offset="50%" stop-color="#00f5ff"/>
      <stop offset="100%" stop-color="#0284c7"/>
    </linearGradient>

    <linearGradient id="edge-emerald" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#34d399"/>
      <stop offset="50%" stop-color="#10b981"/>
      <stop offset="100%" stop-color="#059669"/>
    </linearGradient>

    <linearGradient id="optic-prism" x1="0%" y1="0%" x2="100%" y2="0%">
      <stop offset="0%" stop-color="#0284c7"/>
      <stop offset="45%" stop-color="#00f5ff"/>
      <stop offset="55%" stop-color="#10b981"/>
      <stop offset="100%" stop-color="#f59e0b"/>
    </linearGradient>

    <filter id="subtle-shadow" x="-10%" y="-10%" width="120%" height="120%">
      <feDropShadow dx="0" dy="16" stdDeviation="22" flood-color="#0f172a" flood-opacity="0.18" />
    </filter>
  </defs>

  <rect x="24" y="24" width="976" height="976" rx="220" fill="#ffffff" stroke="#e2e8f0" stroke-width="6" />

  <g clip-path="url(#squircle-clip)">
    <circle cx="512" cy="512" r="390" fill="none" stroke="#f1f5f9" stroke-width="3" stroke-dasharray="8 8" />
    <circle cx="512" cy="512" r="330" fill="none" stroke="#e2e8f0" stroke-width="4" />
    <circle cx="512" cy="512" r="270" fill="none" stroke="#cbd5e1" stroke-width="2" />

    <g transform="translate(512, 520)" filter="url(#subtle-shadow)">
      <path d="
        M -240,-180
        L -180,-340
        L -130,-200
        L 0,-260
        L 130,-200
        L 180,-340
        L 240,-180
        L 290,-40
        L 220,130
        L 130,280
        L 0,330
        L -130,280
        L -220,130
        L -290,-40
        Z
      " fill="#090d16" />

      <polygon points="-240,-180 -180,-340 -130,-200" fill="#1e293b" />
      <polygon points="-210,-200 -180,-320 -150,-220" fill="#00f5ff" opacity="0.85" />
      <polygon points="-180,-340 -170,-380 -165,-340" fill="#0284c7" />

      <polygon points="240,-180 180,-340 130,-200" fill="#1e293b" />
      <polygon points="210,-200 180,-320 150,-220" fill="#10b981" opacity="0.85" />
      <polygon points="180,-340 170,-380 165,-340" fill="#059669" />

      <polygon points="-130,-200 0,-260 0,-130" fill="#334155" />
      <polygon points="130,-200 0,-260 0,-130" fill="#1e293b" />
      <polygon points="-130,-200 0,-130 -140,-80" fill="#475569" />
      <polygon points="130,-200 0,-130 140,-80" fill="#334155" />

      <polygon points="-240,-180 -130,-200 -140,-80 -290,-40" fill="#1e293b" />
      <polygon points="240,-180 130,-200 140,-80 290,-40" fill="#0f172a" />
      <polygon points="-290,-40 -140,-80 -110,60 -220,130" fill="#334155" />
      <polygon points="290,-40 140,-80 110,60 220,130" fill="#1e293b" />

      <polygon points="-140,-80 -60,-80 -85,-15 -140,-35" fill="#050811" />
      <polygon points="-125,-60 -75,-60 -90,-25 -120,-35" fill="url(#edge-cyan)" />
      <polygon points="-102,-60 -92,-60 -95,-25 -98,-25" fill="#090d16" />

      <polygon points="140,-80 60,-80 85,-15 140,-35" fill="#050811" />
      <polygon points="125,-60 75,-60 90,-25 120,-35" fill="url(#edge-emerald)" />
      <polygon points="102,-60 92,-60 95,-25 98,-25" fill="#090d16" />

      <polygon points="0,-130 -60,-80 0,-10" fill="#64748b" />
      <polygon points="0,-130 60,-80 0,-10" fill="#475569" />
      <polygon points="-60,-80 0,-10 -50,60" fill="#334155" />
      <polygon points="60,-80 0,-10 50,60" fill="#1e293b" />

      <polygon points="0,55 -30,85 30,85" fill="#0f172a" />

      <polygon points="-30,85 -110,60 -130,170 -40,165" fill="#94a3b8" opacity="0.9" />
      <polygon points="30,85 110,60 130,170 40,165" fill="#64748b" opacity="0.9" />

      <polygon points="0,110 -40,165 0,220" fill="#cbd5e1" />
      <polygon points="0,110 40,165 0,220" fill="#94a3b8" />
      <polygon points="-40,165 0,220 -130,280" fill="#475569" />
      <polygon points="40,165 0,220 130,280" fill="#334155" />
      <polygon points="0,220 -130,280 0,330" fill="#1e293b" />
      <polygon points="0,220 130,280 0,330" fill="#0f172a" />

      <path d="M -350,195 L 350,195" stroke="url(#optic-prism)" stroke-width="8" stroke-linecap="round" />
    </g>
  </g>
</svg>`;
}

async function main() {
  console.log('1. Generating docs/images/logo.svg and logo.png...');
  const svgContent = buildLogoSvg();
  const svgPath = path.join(targetDir, 'logo.svg');
  fs.writeFileSync(svgPath, svgContent, 'utf8');

  const browser = await puppeteer.launch({
    headless: 'new',
    args: ['--no-sandbox', '--disable-setuid-sandbox', '--window-size=1440,900'],
  });

  const page = await browser.newPage();
  await page.setViewport({ width: 1024, height: 1024, deviceScaleFactor: 1 });
  await page.setContent(`<!DOCTYPE html><html><body style="margin:0;background:transparent;display:flex;align-items:center;justify-content:center;">${svgContent}</body></html>`);
  await page.screenshot({ path: path.join(targetDir, 'logo.png'), omitBackground: true });

  console.log('2. Navigating to EdgeLens Studio at http://localhost:8085...');
  await page.setViewport({ width: 1440, height: 900, deviceScaleFactor: 2 });
  await page.goto('http://localhost:8085', { waitUntil: 'domcontentloaded' });
  await new Promise((r) => setTimeout(r, 1200));

  // Seed sample requests
  await page.evaluate(async () => {
    try {
      await fetch('/proxy?url=' + encodeURIComponent('http://localhost:8085/mock/scenarios/stale-while-revalidate'));
      await fetch('/proxy?url=' + encodeURIComponent('http://localhost:8085/mock/scenarios/product/42'));
      await fetch('/proxy?url=' + encodeURIComponent('http://localhost:8085/mock/scenarios/vary-user-agent'), {
        headers: { 'User-Agent': 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)' }
      });
      await fetch('/proxy?url=' + encodeURIComponent('http://localhost:8085/mock/scenarios/stale-while-revalidate'));
    } catch (e) {}
  });

  await page.reload({ waitUntil: 'domcontentloaded' });
  await new Promise((r) => setTimeout(r, 1500));

  async function selectTab(tabLabel) {
    await page.evaluate((label) => {
      const btns = Array.from(document.querySelectorAll('header nav button'));
      const target = btns.find(b => b.textContent && b.textContent.includes(label));
      if (target) (target).click();
    }, tabLabel);
    await new Promise((r) => setTimeout(r, 1000));
  }

  // 1. edgelens-1.png: Traffic & Explainer
  console.log('Capturing edgelens-1.png...');
  await selectTab('Traffic & Explainer');
  await page.screenshot({ path: path.join(targetDir, 'edgelens-1.png'), fullPage: false });

  // 2. edgelens-2.png: Surrogate Keys
  console.log('Capturing edgelens-2.png...');
  await selectTab('Surrogate Keys');
  await page.screenshot({ path: path.join(targetDir, 'edgelens-2.png'), fullPage: false });

  // 3. edgelens-3.png: Stampede Lab
  console.log('Capturing edgelens-3.png...');
  await selectTab('Stampede Lab');
  const runBtn = await page.$('button.bg-amber-600');
  if (runBtn) {
    await runBtn.click();
    await new Promise((r) => setTimeout(r, 2200));
  }
  await page.screenshot({ path: path.join(targetDir, 'edgelens-3.png'), fullPage: false });

  // 4. edgelens-4.png: Mock Origin
  console.log('Capturing edgelens-4.png...');
  await selectTab('Mock Origin');
  await page.screenshot({ path: path.join(targetDir, 'edgelens-4.png'), fullPage: false });

  // 5. Record Frames for animated demo GIF
  console.log('Recording sequence frames for edgelens_demo.gif...');
  if (!fs.existsSync(framesDir)) fs.mkdirSync(framesDir, { recursive: true });

  let frameIdx = 0;
  const tabs = ['Traffic & Explainer', 'Surrogate Keys', 'Stampede Lab', 'Mock Origin'];
  for (const tabName of tabs) {
    await selectTab(tabName);
    for (let f = 0; f < 4; f++) {
      const framePath = path.join(framesDir, `frame_${String(frameIdx).padStart(4, '0')}.png`);
      await page.screenshot({ path: framePath, fullPage: false });
      frameIdx++;
      await new Promise((r) => setTimeout(r, 250));
    }
  }

  await browser.close();

  console.log(`Captured ${frameIdx} frames. Generating high-quality GIF with ffmpeg...`);
  const gifPath = path.join(targetDir, 'edgelens_demo.gif');
  const palettePath = path.join(framesDir, 'palette.png');

  execSync(`ffmpeg -y -framerate 2 -i "${framesDir}/frame_%04d.png" -vf "fps=2,scale=1280:-1:flags=lanczos,palettegen" "${palettePath}"`, { stdio: 'inherit' });
  execSync(`ffmpeg -y -framerate 2 -i "${framesDir}/frame_%04d.png" -i "${palettePath}" -filter_complex "fps=2,scale=1280:-1:flags=lanczos[x];[x][1:v]paletteuse=dither=bayer:bayer_scale=3" "${gifPath}"`, { stdio: 'inherit' });

  // Cleanup frames
  fs.rmSync(framesDir, { recursive: true, force: true });
  console.log('✓ Successfully generated all screenshots and edgelens_demo.gif!');
}

main().catch(err => {
  console.error(err);
  process.exit(1);
});
