import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

const KB = 1024;
const BUDGETS = { js: 180 * KB, css: 30 * KB };
const dist = fileURLToPath(new URL('../dist/', import.meta.url));
const html = readFileSync(join(dist, 'index.html'), 'utf8');

const attr = (tag, name) => new RegExp(`\\s${name}="([^"]*)"`).exec(tag)?.[1];

const collect = () => {
  const js = new Set();
  const css = new Set();
  for (const [tag] of html.matchAll(/<script\b[^>]*>/g)) {
    if (attr(tag, 'type') === 'module' && attr(tag, 'src')) js.add(attr(tag, 'src'));
  }
  for (const [tag] of html.matchAll(/<link\b[^>]*>/g)) {
    const rel = attr(tag, 'rel');
    if (rel === 'modulepreload') js.add(attr(tag, 'href'));
    if (rel === 'stylesheet') css.add(attr(tag, 'href'));
  }
  return { js, css };
};

const gzipSize = (ref) => gzipSync(readFileSync(join(dist, ref.replace(/^\//, ''))), { level: 9 }).length;

const kb = (bytes) => (bytes / KB).toFixed(1);
const { js, css } = collect();
let failed = false;

for (const [kind, refs] of [
  ['js', js],
  ['css', css],
]) {
  if (refs.size === 0) {
    process.stderr.write(`size: no initial ${kind} found in dist/index.html\n`);
    process.exit(1);
  }
  let total = 0;
  for (const ref of refs) {
    const bytes = gzipSize(ref);
    total += bytes;
    process.stdout.write(`${kb(bytes).padStart(7)} KB  ${ref}\n`);
  }
  const budget = BUDGETS[kind];
  const over = total > budget;
  failed ||= over;
  const verdict = over ? 'OVER' : 'ok';
  process.stdout.write(`size: initial ${kind} is ${kb(total)} KB gzip, budget ${kb(budget)} KB: ${verdict}\n`);
}

if (failed) process.exit(1);
