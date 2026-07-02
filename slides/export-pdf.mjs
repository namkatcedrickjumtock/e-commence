#!/usr/bin/env node

import puppeteer from 'puppeteer-core';
import { PDFDocument } from 'pdf-lib';
import { fileURLToPath } from 'url';
import path from 'path';
import fs from 'fs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const HTML_PATH = path.resolve(__dirname, 'index.html');
const OUTPUT_PATH = path.resolve(__dirname, 'Error-Tracing-GopherCon-EU-2026.pdf');
const CHROME_PATH = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';

const SLIDE_COUNT = 25;
const VIEWPORT_W = 1920;
const VIEWPORT_H = 1080;
const DEVICE_SCALE = 2;

function pad(n) {
  return String(n).padStart(2, '0');
}

async function main() {
  if (!fs.existsSync(HTML_PATH)) {
    console.error(`HTML file not found: ${HTML_PATH}`);
    process.exit(1);
  }

  console.log('┌──────────────────────────────────────────┐');
  console.log('│  Error Tracing — PDF Export              │');
  console.log('│  GopherCon Europe 2026                   │');
  console.log('└──────────────────────────────────────────┘');
  console.log();

  if (!fs.existsSync(CHROME_PATH)) {
    console.error(`Chrome not found at: ${CHROME_PATH}`);
    process.exit(1);
  }

  console.log('Launching headless browser...');
  const browser = await puppeteer.launch({
    headless: 'new',
    executablePath: CHROME_PATH,
    args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage'],
  });

  const page = await browser.newPage();
  await page.setViewport({ width: VIEWPORT_W, height: VIEWPORT_H, deviceScaleFactor: DEVICE_SCALE });
  await page.emulateMediaFeatures([{ name: 'prefers-reduced-motion', value: 'reduce' }]);

  console.log('Opening HTML...');
  await page.goto(`file://${HTML_PATH}`, { waitUntil: 'domcontentloaded', timeout: 30000 });
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
        if (j === idx) s.setAttribute('data-deck-active', '');
        else s.removeAttribute('data-deck-active');
      });
    }, i);

    await new Promise(r => setTimeout(r, 200));

    await page.evaluate(() => {
      const deck = document.querySelector('deck-stage');
      const shadow = deck && deck.shadowRoot;
      if (shadow) {
        const overlay = shadow.querySelector('.overlay');
        if (overlay) overlay.removeAttribute('data-visible');
      }
    });

    const screenshot = await page.screenshot({
      type: 'png',
      clip: { x: 0, y: 0, width: VIEWPORT_W, height: VIEWPORT_H },
    });

    screenshots.push(screenshot);
    process.stdout.write('done\n');
  }

  await browser.close();

  console.log();
  console.log('Building PDF...');

  const pdfDoc = await PDFDocument.create();
  pdfDoc.setTitle('Error Tracing: Lessons Learned From the Trenches');
  pdfDoc.setAuthor('Namkat Cedrick');
  pdfDoc.setSubject('GopherCon Europe 2026 · Berlin');

  // PDF page size at 72 dpi: 1920×1080 maps to 13.333in × 7.5in = 960pt × 540pt
  const PAGE_W = 960;
  const PAGE_H = 540;

  for (let i = 0; i < screenshots.length; i++) {
    process.stdout.write(`  Embedding slide ${pad(i + 1)}/${screenshots.length} ... `);
    const pngImage = await pdfDoc.embedPng(screenshots[i]);
    const page = pdfDoc.addPage([PAGE_W, PAGE_H]);
    page.drawImage(pngImage, { x: 0, y: 0, width: PAGE_W, height: PAGE_H });
    process.stdout.write('done\n');
  }

  const pdfBytes = await pdfDoc.save();
  fs.writeFileSync(OUTPUT_PATH, pdfBytes);

  const stats = fs.statSync(OUTPUT_PATH);
  console.log();
  console.log('┌──────────────────────────────────────────┐');
  console.log(`│  PDF saved (${(stats.size / 1024 / 1024).toFixed(1)} MB)                 │`);
  console.log(`│  ${path.basename(OUTPUT_PATH)}  │`);
  console.log('└──────────────────────────────────────────┘');
}

main().catch((err) => {
  console.error('\nExport failed:', err);
  process.exit(1);
});
