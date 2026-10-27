import { existsSync, readFileSync } from 'node:fs';
import { join, posix } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

const KB = 1024;
const BUDGETS = { js: 180 * KB, css: 30 * KB };
const dist = fileURLToPath(new URL('../dist/', import.meta.url));
const html = readFileSync(join(dist, 'index.html'), 'utf8');

const STATIC_IMPORT =
  /(?:^|[^\w$.])(?:import|export)\s*(?:[\w$]+\s*,?\s*)?(?:\*\s*(?:as\s+[\w$]+\s*)?|\{[^{}]*\}\s*)?(?:from\s*)?["']([^"']+)["']/g;

const attr = (tag, name) => new RegExp(`\\s${name}="([^"]*)"`, 'i').exec(tag)?.[1];

const fail = (message) => {
  process.stderr.write(`size: ${message}\n`);
  process.exit(1);
};

const checkInline = () => {
  for (const [tag] of html.matchAll(/<script\b[^>]*>/gi)) {
    if (!attr(tag, 'src')) fail(`dist/index.html has an inline script, which the 'self' CSP blocks: ${tag}`);
  }
  if (/<style\b/i.test(html)) fail(`dist/index.html has an inline <style>, which the 'self' CSP blocks`);
  if (/\sstyle="/i.test(html)) fail(`dist/index.html has a style attribute, which the 'self' CSP blocks`);
};

const entries = () => {
  const js = new Set();
  const css = new Set();
  for (const [tag] of html.matchAll(/<script\b[^>]*>/gi)) {
    if (attr(tag, 'type') === 'module' && attr(tag, 'src')) js.add(attr(tag, 'src'));
  }
  for (const [tag] of html.matchAll(/<link\b[^>]*>/gi)) {
    const rel = attr(tag, 'rel');
    if (rel === 'modulepreload') js.add(attr(tag, 'href'));
    if (rel === 'stylesheet') css.add(attr(tag, 'href'));
  }
  return { js, css };
};

const fileOf = (ref) => join(dist, ref.replace(/^\//, ''));

const staticImports = (ref) => {
  const code = readFileSync(fileOf(ref), 'utf8');
  const found = [];
  for (const [, spec] of code.matchAll(STATIC_IMPORT)) {
    if (!/^\.{0,2}\//.test(spec)) continue;
    const target = spec.startsWith('/') ? spec : posix.join(posix.dirname(ref), spec);
    if (!existsSync(fileOf(target))) fail(`${ref} imports ${spec}, which is not in dist/`);
    found.push(target);
  }
  return found;
};

const followGraph = (roots) => {
  const seen = new Set();
  const queue = [...roots];
  while (queue.length > 0) {
    const ref = queue.shift();
    if (seen.has(ref)) continue;
    seen.add(ref);
    queue.push(...staticImports(ref));
  }
  return seen;
};

const gzipSize = (ref) => gzipSync(readFileSync(fileOf(ref)), { level: 9 }).length;

const kb = (bytes) => (bytes / KB).toFixed(1);

checkInline();
const roots = entries();
const initial = { js: followGraph(roots.js), css: roots.css };
let failed = false;

for (const kind of ['js', 'css']) {
  const refs = initial[kind];
  if (refs.size === 0) fail(`no initial ${kind} found from dist/index.html`);
  const sizes = [...refs].map((ref) => [ref, gzipSize(ref)]).sort((a, b) => b[1] - a[1]);
  let total = 0;
  for (const [ref, bytes] of sizes) {
    total += bytes;
    process.stdout.write(`${kb(bytes).padStart(7)} KB  ${ref}\n`);
  }
  const budget = BUDGETS[kind];
  const over = total > budget;
  failed ||= over;
  const verdict = over ? 'OVER' : 'ok';
  process.stdout.write(
    `size: initial ${kind} is ${kb(total)} KB gzip in ${refs.size} files, budget ${kb(budget)} KB: ${verdict}\n`,
  );
}

if (failed) process.exit(1);
