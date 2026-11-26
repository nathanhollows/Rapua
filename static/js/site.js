(() => {
  // Hills are fractions of the box so a resize redraws the same land. A fitted
  // (small) map gets fewer hills and lines to keep the rings as far apart as on
  // a full-width ground.
  function drawTopo(svg, fit = false) {
    const w = svg.clientWidth, h = svg.clientHeight;
    if (!w || !h) return;
    svg.hills ||= Array.from({ length: fit && Math.hypot(w, h) < 900 ? 3 : 5 }, () => ({ x: Math.random(), y: Math.random(), a: .5 + Math.random() * .6, s: .16 + Math.random() * .16 }));
    svg.roll ||= Array.from({ length: 3 }, () => ({ fx: .003 + Math.random() * .005, fy: .003 + Math.random() * .005, p: Math.random() * 6.3, q: Math.random() * 6.3 }));
    // A coarser grid on phones cuts about 40% of the work on the slowest devices.
    const CELL = w < 640 ? 11 : 7, nx = Math.ceil(w / CELL) + 1, ny = Math.ceil(h / CELL) + 1, span = Math.hypot(w, h);
    const f = [];
    let lo = Infinity, hi = -Infinity;
    for (let j = 0; j < ny; j++) {
      const row = [];
      for (let i = 0; i < nx; i++) {
        let v = 0;
        for (const hl of svg.hills) {
          const dx = i * CELL - hl.x * w, dy = j * CELL - hl.y * h, s = hl.s * span;
          v += hl.a * Math.exp(-(dx * dx + dy * dy) / (2 * s * s));
        }
        for (const r of svg.roll) v += .09 * Math.sin(i * CELL * r.fx + r.p) * Math.cos(j * CELL * r.fy + r.q);
        row.push(v); lo = Math.min(lo, v); hi = Math.max(hi, v);
      }
      f.push(row);
    }
    const EDGES = [[], [3, 2], [2, 1], [3, 1], [0, 1], [0, 3, 2, 1], [0, 2], [0, 3], [0, 3], [0, 2], [0, 1, 3, 2], [0, 1], [3, 1], [2, 1], [3, 2], []];
    const LEVELS = fit ? Math.max(8, Math.min(24, Math.round(span / 60))) : 24;
    // Only the first full-size draw animates; redraws and reduced motion skip it.
    const rise = !svg.drawn && !fit && !matchMedia('(prefers-reduced-motion: reduce)').matches;
    // One task per level so no step blocks the page; a redraw swaps in at the
    // end so it never shows half a map.
    const run = svg.topoRun = {};
    svg.drawn = true;
    svg.toggleAttribute('data-rise', rise);
    svg.setAttribute('viewBox', `0 0 ${w} ${h}`);
    if (rise) svg.replaceChildren();
    const ready = [], t0 = performance.now();
    let n = LEVELS - 1, shown = 0;
    const step = () => {
      if (svg.topoRun !== run || !svg.isConnected) return;
      const lv = lo + (hi - lo) * n / LEVELS;
      let d = '';
      for (let j = 0; j < ny - 1; j++) for (let i = 0; i < nx - 1; i++) {
        const a = f[j][i], b = f[j][i + 1], c = f[j + 1][i + 1], e = f[j + 1][i];
        const ed = EDGES[(a > lv ? 8 : 0) | (b > lv ? 4 : 0) | (c > lv ? 2 : 0) | (e > lv ? 1 : 0)];
        if (!ed.length) continue;
        const x = i * CELL, y = j * CELL, t = (p, q) => (lv - p) / (q - p);
        const pts = [() => [x + t(a, b) * CELL, y], () => [x + CELL, y + t(b, c) * CELL], () => [x + t(e, c) * CELL, y + CELL], () => [x, y + t(a, e) * CELL]];
        for (let k = 0; k < ed.length; k += 2) {
          const p = pts[ed[k]](), q = pts[ed[k + 1]]();
          d += `M${p[0].toFixed(1)} ${p[1].toFixed(1)}L${q[0].toFixed(1)} ${q[1].toFixed(1)}`;
        }
      }
      if (d) {
        const line = document.createElementNS('http://www.w3.org/2000/svg', 'path');
        line.setAttribute('d', d);
        if (n % 5 === 0) line.setAttribute('class', 'site-topo-index');
        if (rise) {
          // Holds the fade to one level per 22ms however fast levels compute.
          line.style.animationDelay = Math.max(0, shown * 22 - (performance.now() - t0)) + 'ms';
          svg.append(line);
        } else ready.push(line);
        shown++;
      }
      if (--n >= 1) setTimeout(step, 0);
      else if (!rise) svg.replaceChildren(...ready);
    };
    step();
  }

  // Centripetal, so the curve has no cusps or loops between close points.
  function spline(p) {
    if (p.length < 2) return '';
    const ext = [[2 * p[0][0] - p[1][0], 2 * p[0][1] - p[1][1]], ...p, [2 * p.at(-1)[0] - p.at(-2)[0], 2 * p.at(-1)[1] - p.at(-2)[1]]];
    const dist = (a, b) => Math.max(1e-3, Math.hypot(b[0] - a[0], b[1] - a[1]) ** .5);
    let d = `M${p[0][0].toFixed(1)} ${p[0][1].toFixed(1)}`;
    for (let i = 1; i < ext.length - 2; i++) {
      const [p0, p1, p2, p3] = [ext[i - 1], ext[i], ext[i + 1], ext[i + 2]];
      const t01 = dist(p0, p1), t12 = dist(p1, p2), t23 = dist(p2, p3);
      const m1 = [0, 1].map(k => p2[k] - p1[k] + t12 * ((p1[k] - p0[k]) / t01 - (p2[k] - p0[k]) / (t01 + t12)));
      const m2 = [0, 1].map(k => p2[k] - p1[k] + t12 * ((p3[k] - p2[k]) / t23 - (p3[k] - p1[k]) / (t12 + t23)));
      const c1 = [p1[0] + m1[0] / 3, p1[1] + m1[1] / 3], c2 = [p2[0] - m2[0] / 3, p2[1] - m2[1] / 3];
      d += ` C${c1.map(v => v.toFixed(1))} ${c2.map(v => v.toFixed(1))} ${p2.map(v => v.toFixed(1))}`;
    }
    return d;
  }

  function drawTrail(list) {
    const lr = list.getBoundingClientRect();
    // The nudge stops a straight column of stickers reading as a ruler.
    const pts = [...list.querySelectorAll('.site-st')].map((s, i) => {
      const r = s.getBoundingClientRect();
      return [r.left - lr.left + r.width / 2 + (i % 2 ? 6 : -6), r.top - lr.top + r.height / 2];
    });
    list.querySelector(':scope > .site-trail')?.remove();
    list.insertAdjacentHTML('afterbegin', `<svg class="site-trail" viewBox="0 0 ${lr.width} ${lr.height}" aria-hidden="true"><path d="${spline(pts)}"/></svg>`);
  }

  // Boosted swaps keep the window alive, so pages register teardown for their
  // loops, timers and listeners here.
  const leaving = [];
  window.siteLeave = fn => leaving.push(fn);
  document.addEventListener('htmx:beforeSwap', e => {
    if (e.detail.target !== document.body || !e.detail.shouldSwap) return;
    for (const fn of leaving.splice(0)) {
      try { fn(); } catch (err) { console.error('siteLeave', err); }
    }
  });

  window.siteTopo = drawTopo;
  window.siteSpline = spline;

  // The first draw waits for a paint so computing contours never delays the first view or tap.
  const watch = (el, draw, box = el) => {
    let t, f, first = true;
    const o = new ResizeObserver(() => {
      clearTimeout(t);
      if (!first) { t = setTimeout(() => draw(el), 60); return; }
      first = false;
      f = requestAnimationFrame(() => { t = setTimeout(() => draw(el), 0); });
    });
    o.observe(box);
    window.siteLeave(() => { o.disconnect(); clearTimeout(t); cancelAnimationFrame(f); });
  };
  function start() {
    document.querySelectorAll('[data-topo]').forEach(svg => watch(svg, drawTopo, svg.parentElement));
    document.querySelectorAll('[data-trail]').forEach(list => watch(list, drawTrail));
  }
  // Loaded once in the head, so each boosted page has to be started by hand.
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start);
  else start();
  // The swap leaves focus on a link that no longer exists.
  document.addEventListener('htmx:afterSettle', e => {
    if (e.detail.target !== document.body) return;
    start();
    const main = document.getElementById('main');
    if (main) { main.tabIndex = -1; main.focus({ preventScroll: true }); }
  });
})();
