#!/usr/bin/env node

import puppeteer from 'puppeteer-core';
import PptxGenJS from 'pptxgenjs';
import { fileURLToPath } from 'url';
import path from 'path';
import fs from 'fs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const HTML_PATH = path.resolve(__dirname, 'index.html');
const OUTPUT_PATH = path.resolve(__dirname, 'Error-Tracing-GopherCon-EU-2026.pptx');
const CHROME_PATH = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';

const SLIDE_COUNT = 25;
const VIEWPORT_W = 1920;
const VIEWPORT_H = 1080;
const DEVICE_SCALE = 2; // renders at 3840×2160 for crisp text on HiDPI displays
const CAPTURE_W = VIEWPORT_W * DEVICE_SCALE;
const CAPTURE_H = VIEWPORT_H * DEVICE_SCALE;

function pad(n) {
  return String(n).padStart(2, '0');
}

async function main() {
  if (!fs.existsSync(HTML_PATH)) {
    console.error(`HTML file not found: ${HTML_PATH}`);
    console.error('Run this script from the slides/ directory.');
    process.exit(1);
  }

  console.log('┌──────────────────────────────────────────┐');
  console.log('│  Error Tracing — PPTX Export             │');
  console.log('│  GopherCon Europe 2026                   │');
  console.log('└──────────────────────────────────────────┘');
  console.log();

  console.log('Launching headless browser...');

  if (!fs.existsSync(CHROME_PATH)) {
    console.error(`Chrome not found at: ${CHROME_PATH}`);
    console.error('Install Google Chrome or update CHROME_PATH in the script.');
    process.exit(1);
  }

  const browser = await puppeteer.launch({
    headless: 'new',
    executablePath: CHROME_PATH,
    args: [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      '--disable-dev-shm-usage',
    ],
  });

  const page = await browser.newPage();
  await page.setViewport({ width: VIEWPORT_W, height: VIEWPORT_H, deviceScaleFactor: DEVICE_SCALE });

  await page.emulateMediaFeatures([
    { name: 'prefers-reduced-motion', value: 'reduce' },
  ]);

  console.log('Opening HTML...');
  await page.goto(`file://${HTML_PATH}`, {
    waitUntil: 'domcontentloaded',
    timeout: 30000,
  });

  await page.waitForSelector('deck-stage', { timeout: 10000 });
  await page.evaluate(() => customElements.whenDefined('deck-stage'));
  await page.evaluate(() => document.fonts.ready);

  await page.evaluate(() => {
    const deck = document.querySelector('deck-stage');
    if (deck) deck.setAttribute('noscale', '');
  });

  await new Promise(r => setTimeout(r, 500));

  const slideLabels = await page.evaluate(() => {
    const deck = document.querySelector('deck-stage');
    return Array.from(deck.querySelectorAll('section')).map((s, idx) =>
      s.getAttribute('data-label') || `Slide ${idx + 1}`,
    );
  });

  if (slideLabels.length !== SLIDE_COUNT) {
    console.warn(`Warning: expected ${SLIDE_COUNT} slides, found ${slideLabels.length}`);
  }

  console.log(`Found ${slideLabels.length} slides`);
  console.log();

  const screenshots = [];

  for (let i = 0; i < slideLabels.length; i++) {
    const label = slideLabels[i];
    process.stdout.write(`  [${pad(i + 1)}/${slideLabels.length}] ${label} ... `);

    await page.evaluate((idx) => {
      const deck = document.querySelector('deck-stage');
      const sections = deck.querySelectorAll('section');
      sections.forEach((s, j) => {
        if (j === idx) {
          s.setAttribute('data-deck-active', '');
        } else {
          s.removeAttribute('data-deck-active');
        }
      });
    }, i);

    await new Promise(r => setTimeout(r, 200));

    // Hide the navigation overlay so it never bleeds into the capture.
    await page.evaluate(() => {
      const deck = document.querySelector('deck-stage');
      const shadow = deck && deck.shadowRoot;
      if (shadow) {
        const overlay = shadow.querySelector('.overlay');
        if (overlay) overlay.removeAttribute('data-visible');
      }
    });

    // Capture the full 1920×1080 viewport via an explicit clip rect.
    // This is more reliable than elementHandle.screenshot() for slotted
    // Web Component children whose bounding box can be reported in the
    // wrong coordinate space, causing content to be cut off.
    const screenshot = await page.screenshot({
      type: 'png',
      clip: { x: 0, y: 0, width: VIEWPORT_W, height: VIEWPORT_H },
    });

    screenshots.push(screenshot);
    process.stdout.write(`${CAPTURE_W}x${CAPTURE_H}\n`);
  }

  await browser.close();

  console.log();
  console.log('Building PPTX...');

  const pptx = new PptxGenJS();
  pptx.defineLayout({ name: 'WIDE', width: 13.333, height: 7.5 });
  pptx.layout = 'WIDE';
  pptx.author = 'Namkat Cedrick';
  pptx.title = 'Error Tracing: Lessons Learned From the Trenches';
  pptx.subject = 'GopherCon Europe 2026 · Berlin';

  for (let i = 0; i < screenshots.length; i++) {
    const slide = pptx.addSlide();
    const b64 = screenshots[i].toString('base64');
    slide.addImage({
      data: `data:image/png;base64,${b64}`,
      x: 0,
      y: 0,
      w: 13.333,
      h: 7.5,
    });
  }

  await pptx.writeFile({ fileName: OUTPUT_PATH });

  const stats = fs.statSync(OUTPUT_PATH);
  console.log();
  console.log('┌──────────────────────────────────────────┐');
  console.log(`│  ✅  PPTX saved (${(stats.size / 1024 / 1024).toFixed(1)} MB)             │`);
  console.log(`│  ${OUTPUT_PATH}        │`);
  console.log('└──────────────────────────────────────────┘');
}

main().catch((err) => {
  console.error('\nExport failed:', err);
  process.exit(1);
});
