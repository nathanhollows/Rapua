(() => {
  const $ = id => document.getElementById(id);
  const main = $('main'), form = $('join'), input = $('code'), tilesBox = $('tiles'), err = $('err'), go = $('go'), hero = $('hero'), tip = $('tip');
  const CODE_LENGTH = Number(form.dataset.length);
  const tiles = [...tilesBox.querySelectorAll('.home-tile')];
  const goLabel = go.textContent;
  const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
  const returning = main.dataset.state === 'returning';
  const ac = new AbortController(), observers = [];
  let gone = false;
  window.siteLeave?.(() => { gone = true; ac.abort(); observers.forEach(o => o.disconnect()); });

  const ICONS = {
    qr: '<path d="M17 12v4a1 1 0 0 1-1 1h-4"/><path d="M17 3h2a2 2 0 0 1 2 2v2"/><path d="M17 8V7"/><path d="M21 17v2a2 2 0 0 1-2 2h-2"/><path d="M3 7V5a2 2 0 0 1 2-2h2"/><path d="M7 17h.01"/><path d="M7 21H5a2 2 0 0 1-2-2v-2"/><rect x="7" y="7" width="5" height="5" rx="1"/>',
    pin: '<path d="M20 10c0 4.993-5.539 10.193-7.399 11.799a1 1 0 0 1-1.202 0C9.539 20.193 4 14.993 4 10a8 8 0 0 1 16 0"/><circle cx="12" cy="10" r="3"/>',
    camera: '<path d="M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3l-2.5-3z"/><circle cx="12" cy="13" r="3"/>',
    quiz: '<circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><path d="M12 17h.01"/>',
    keypad: '<path d="M5 5h.01M12 5h.01M19 5h.01M5 12h.01M12 12h.01M19 12h.01M5 19h.01M12 19h.01M19 19h.01" stroke-width="3.4"/>',
    sort: '<path d="m3 16 4 4 4-4"/><path d="M7 20V4"/><path d="m21 8-4-4-4 4"/><path d="M17 4v16"/>',
  };
  // Each tip is set somewhere different to say any space can hold a quest.
  const TIPS = {
    qr: ['Scan the code under the dinosaur\'s chin', 'At a museum'],
    pin: ['Stand exactly where the old pier ended', 'Along a waterfront'],
    camera: ['Get a selfie with the angriest gargoyle', 'Around a campus'],
    quiz: ['How many lions guard the library?', 'In a library'],
    keypad: ['Crack the lock on the captain\'s chest', 'On a school trip'],
    sort: ['Put the prime ministers in order', 'In a classroom'],
  };
  // Semantic colours so every theme keeps the icon legible.
  const COLORS = ['primary', 'secondary', 'accent', 'primary', 'secondary', 'accent'];
  const STICKERS = [
    { k: 'qr', x: 17, y: 30, px: 15, py: 8, r: -12, s: 76 },
    { k: 'camera', x: 88, y: 26, px: 84, py: 12, r: 10, s: 70 },
    { k: 'quiz', x: 7, y: 60, px: 20, py: 93, r: 8, s: 64 },
    { k: 'pin', x: 93, y: 62, px: 80, py: 87, r: -9, s: 80 },
    { k: 'keypad', x: 25, y: 86, r: 14, s: 60, desk: true },
    { k: 'sort', x: 72, y: 84, r: -6, s: 66, desk: true },
  ].map(st => ({ ...st, px: st.px ?? st.x, py: st.py ?? st.y }));
  // Percent of the sticker box. Points may sit off screen so the line enters and leaves the hero.
  const COURSE = {
    desk: [[-12, -16], 'qr', 'quiz', 'keypad', [36, 124], [64, 124], 'sort', 'pin', 'camera', 'loop', [132, 8]],
    phone: [[22, -14], 'qr', [-26, 48], 'quiz', [50, 99], 'pin', [126, 48], 'camera', [78, -14]],
  };
  const CITIES = ['dunedin', 'wellington', 'christchurch', 'auckland'];

  const svg = k => `<svg viewBox="0 0 24 24" aria-hidden="true">${ICONS[k]}</svg>`;
  const stickerEls = STICKERS.map((st, i) => {
    const b = document.createElement('button');
    b.type = 'button';
    b.className = 'home-st';
    b.dataset.k = st.k;
    // Tips are pointer-only, so keyboard and screen reader users skip to the code.
    b.tabIndex = -1;
    b.setAttribute('aria-hidden', 'true');
    const c = COLORS[i % COLORS.length];
    b.style.cssText = `--x:${st.x}%;--y:${st.y}%;--px:${st.px}%;--py:${st.py}%;--r:${st.r}deg;--s:${st.s}px;--d:${(-i * .7).toFixed(1)}s;--c:var(--color-${c});--cc:var(--color-${c}-content)`;
    if (st.desk) b.dataset.deskOnly = '';
    b.innerHTML = `<span class="home-st-body">${svg(st.k)}</span>`;
    $('stickers').appendChild(b);
    return b;
  });
  document.querySelectorAll('[data-ic]').forEach(el => el.insertAdjacentHTML('afterbegin', svg(el.dataset.ic)));

  const city = CITIES[Math.floor(Math.random() * CITIES.length)];
  fetch(`/static/js/maps/${city}.json`).then(r => r.json()).then(m => {
    for (const k of ['water', 'river', 'coast', 'street', 'major']) $('m-' + k).setAttribute('d', m[k] || '');
  }).catch(() => {});

  const spline = window.siteSpline;
  const masked = (el, ds, prefix) => {
    const w = el.clientWidth, h = el.clientHeight;
    return '<defs>' + ds.map((d, i) => `<mask id="${prefix}${i}" maskUnits="userSpaceOnUse" x="${-w}" y="${-h}" width="${w * 3}" height="${h * 3}"><path class="home-rv" pathLength="1" d="${d}"/></mask>`).join('') + '</defs>'
      + ds.map((d, i) => `<path class="home-ink" d="${d}" mask="url(#${prefix}${i})"/>`).join('');
  };

  let revealed = false;
  function drawTrail() {
    const phone = innerWidth < 640, w = hero.clientWidth, h = hero.clientHeight;
    // Relative to the sticker box so a wide screen keeps the trail around the content.
    const box = $('stickers'), bx = box.offsetLeft, bw = box.offsetWidth;
    const at = ([x, y]) => [bx + x / 100 * bw, y / 100 * h];
    const spot = k => {
      const st = STICKERS.find(s => s.k === k);
      return phone ? at([st.px, st.py]) : at([st.x, st.y]);
    };
    const point = c => typeof c === 'string' ? spot(c) : at(c);
    const course = COURSE[phone ? 'phone' : 'desk'], r = phone ? 20 : 22, pts = [];
    course.forEach((c, i) => {
      if (c !== 'loop') { pts.push(point(c)); return; }
      const a = pts.at(-1), b = point(course[i + 1]);
      const len = Math.hypot(b[0] - a[0], b[1] - a[1]), dx = (b[0] - a[0]) / len, dy = (b[1] - a[1]) / len;
      const m = [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2];
      const out = dx > 0 ? 1 : -1, nx = dy * out, ny = -dx * out;
      // A prolate cycloid period has no corner where the loop rejoins the line.
      const k = 2.6, sc = r / 1.4;
      for (let s = 0; s <= 32; s++) {
        const t = -Math.PI + s / 16 * Math.PI, along = sc * (t - k * Math.sin(t)), up = sc * k * (1 + Math.cos(t));
        pts.push([m[0] + dx * along + nx * up, m[1] + dy * along + ny * up]);
      }
    });
    const trail = $('trail');
    trail.setAttribute('viewBox', `0 0 ${w} ${h}`);
    trail.innerHTML = masked(hero, [spline(pts)], 'rv');
    if (revealed || reduced) {
      trail.setAttribute('data-done', '');
      stickerEls.forEach(b => b.setAttribute('data-in', ''));
      return;
    }
    revealed = true;
    // getPointAtLength is slow on a looping path, so sample once per STEP and lazily, a few a frame,
    // to avoid stalling before the walk starts. Stickers are about 64px wide, so 12px misses none.
    const SPEED = phone ? 380 : 520, STEP = 12;
    const seen = q => q.x >= 0 && q.x <= w && q.y >= 0 && q.y <= h;
    const p = trail.querySelector('.home-ink'), rv = trail.querySelector('.home-rv'), len = p.getTotalLength();
    // Off-screen stretches run 14x faster so nobody waits on line they cannot see.
    const marks = [p.getPointAtLength(0)], clock = [0];
    let prev = seen(marks[0]);
    const sampleTo = t => {
      while (clock.at(-1) <= t && (marks.length - 1) * STEP < len) {
        const q = p.getPointAtLength(Math.min(marks.length * STEP, len)), vis = seen(q);
        marks.push(q);
        clock.push(clock.at(-1) + STEP / SPEED / (prev || vis ? 1 : 14));
        prev = vis;
      }
    };
    const hr = hero.getBoundingClientRect();
    let waiting = stickerEls.filter(b => b.offsetParent).map(b => {
      const br = b.getBoundingClientRect();
      return { b, x0: br.left - hr.left - 4, y0: br.top - hr.top - 4, x1: br.right - hr.left + 4, y1: br.bottom - hr.top + 4 };
    });
    const t0 = performance.now();
    let swept = 0, i = 0;
    (function watch(now) {
      const t = (now - t0) / 1000;
      sampleTo(t);
      while (i < clock.length - 1 && clock[i + 1] <= t) i++;
      const into = i < clock.length - 1 ? (t - clock[i]) / (clock[i + 1] - clock[i]) : 0;
      const f = Math.max(0, Math.min(1, (i + into) * STEP / len));
      rv.style.strokeDashoffset = 1 - f;
      // Sweeps since the last frame so a dropped frame cannot skip a sticker.
      for (let k = swept; k <= Math.min(marks.length - 1, f * len / STEP); k++) {
        const q = marks[k];
        waiting = waiting.filter(s => {
          if (q.x < s.x0 || q.x > s.x1 || q.y < s.y0 || q.y > s.y1) return true;
          s.b.setAttribute('data-in', ''); hop(s.b);
          return false;
        });
      }
      swept = Math.floor(f * len / STEP) + 1;
      if (f < 1 && !gone) requestAnimationFrame(watch);
      else { waiting.forEach(s => s.b.setAttribute('data-in', '')); settle(trail); }
    })(t0);
  }

  // A marching dash repaints the map every frame, so a drawn trail stops. It eases out at the march's
  // own speed (26 units per 1.6s; the curve starts at twice its average, so 16 units over 2s) to avoid a jolt.
  function settle(svg) {
    const inks = [...svg.querySelectorAll('.home-ink')];
    const from = inks.map(p => parseFloat(getComputedStyle(p).strokeDashoffset) || 0);
    inks.forEach((p, i) => { p.style.strokeDashoffset = from[i]; });
    svg.setAttribute('data-done', '');
    if (reduced) return;
    inks.forEach((p, i) => p.animate([{ strokeDashoffset: from[i] }, { strokeDashoffset: from[i] - 16.25 }], { duration: 2000, easing: 'cubic-bezier(.3, .6, .5, 1)', fill: 'forwards' }));
  }

  function hop(el) {
    if (reduced) return;
    el.removeAttribute('data-hop'); void el.offsetWidth; el.setAttribute('data-hop', '');
  }

  let tipFor = null;
  function showTip(b) {
    if (tipFor && tipFor !== b) tipFor.removeAttribute('data-tip');
    tipFor = b; b.setAttribute('data-tip', '');
    const [t, p] = TIPS[b.dataset.k];
    tip.querySelector('strong').textContent = t;
    tip.querySelector('span').textContent = p;
    const r = b.getBoundingClientRect(), hr = hero.getBoundingClientRect(), w = tip.offsetWidth, h = tip.offsetHeight;
    const above = r.top - hr.top - h - 12;
    tip.style.left = Math.max(10, Math.min(hr.width - w - 10, r.left + r.width / 2 - hr.left - w / 2)) + 'px';
    tip.style.top = (above > 8 ? above : r.bottom - hr.top + 12) + 'px';
    tip.setAttribute('data-on', '');
  }
  function hideTip() { tip.removeAttribute('data-on'); if (tipFor) tipFor.removeAttribute('data-tip'); tipFor = null; }
  stickerEls.forEach(b => {
    b.addEventListener('pointerenter', e => { if (e.pointerType === 'mouse') showTip(b); });
    b.addEventListener('pointerleave', e => { if (e.pointerType === 'mouse') hideTip(); });
    // Touch has no hover.
    b.addEventListener('click', e => { e.stopPropagation(); hop(b); tipFor === b ? hideTip() : showTip(b); });
  });
  document.addEventListener('click', hideTip, { signal: ac.signal });

  const ABC = 'ABCDEFGHJKLMNPRSTUVWXYZ';
  let shownCode = '';
  function flipTo(i, letter) {
    const span = tiles[i].querySelector('span');
    if (reduced || !letter) { span.textContent = letter; return; }
    let n = 0;
    const tick = () => {
      span.removeAttribute('data-flap'); void span.offsetWidth; span.setAttribute('data-flap', '');
      if (n++ < 4 && input.value[i] === letter) { span.textContent = ABC[Math.floor(Math.random() * ABC.length)]; setTimeout(tick, 42); }
      else {
        span.textContent = input.value[i] || '';
        tiles[i].removeAttribute('data-landed'); void tiles[i].offsetWidth; tiles[i].setAttribute('data-landed', '');
      }
    };
    tick();
  }
  function render() {
    const v = input.value;
    tiles.forEach((t, i) => {
      if ((v[i] || '') !== (shownCode[i] || '')) flipTo(i, v[i] || '');
      t.toggleAttribute('data-filled', !!v[i]);
      t.toggleAttribute('data-next', i === Math.min(v.length, CODE_LENGTH - 1) && !v[i]);
    });
    if (v.length > shownCode.length) hop(stickerEls[(v.length - 1) % stickerEls.length]);
    shownCode = v;
  }

  const ARROW = '<svg class="home-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12h14"/><path d="m12 5 7 7-7 7"/></svg>';
  function setButton(state) {
    go.dataset.btn = state;
    go.removeAttribute('data-go');
    go.setAttribute('aria-disabled', state === 'ready' ? 'false' : 'true');
    tilesBox.toggleAttribute('data-ok', state === 'ready');
    // #err announces failures; the other states only show as button text.
    $('code-status').textContent = { checking: 'Checking the game code.', ready: 'Game code found. Press Start to join.' }[state] || '';
    go.innerHTML = {
      idle: goLabel,
      checking: '<span class="home-spin" aria-hidden="true"></span> Checking',
      ready: 'Start ' + ARROW,
      error: 'Game code not found',
    }[state];
  }
  function clearError() { tilesBox.removeAttribute('data-error'); tilesBox.removeAttribute('data-shake'); err.textContent = ''; }
  function showError(message) {
    clearError();
    tilesBox.setAttribute('data-error', '');
    // Shake after the colour settles so the two never repaint in the same frames.
    setTimeout(() => { if (tilesBox.hasAttribute('data-error')) tilesBox.setAttribute('data-shake', ''); }, 160);
    err.textContent = message;
    setButton('error');
  }
  function burst(el, count, pad, travel) {
    if (reduced) return;
    const r = el.getBoundingClientRect(), cx = r.left + r.width / 2, cy = r.top + r.height / 2;
    for (let k = 0; k < count; k++) {
      const t = k / count * Math.PI * 2 + .2, c = Math.cos(t), s = Math.sin(t);
      const ray = document.createElement('i');
      ray.className = 'home-ray';
      ray.style.cssText = `left:${cx + c * (r.width / 2 + pad)}px;top:${cy + s * (r.height / 2 + pad)}px;--a:${t + Math.PI / 2}rad;--dx:${(c * travel).toFixed(1)}px;--dy:${(s * travel).toFixed(1)}px`;
      document.body.appendChild(ray);
      setTimeout(() => ray.remove(), 500);
    }
  }

  const joining = () => main.toggleAttribute('data-joining', returning && (document.activeElement === input || !!input.value));
  input.addEventListener('focus', joining);
  input.addEventListener('blur', joining);
  // Strips the display dash and anything else pasted in.
  input.addEventListener('input', () => {
    input.value = input.value.toUpperCase().replace(/[^A-Z]/g, '').slice(0, CODE_LENGTH);
    clearError(); setButton('idle'); render(); joining();
    if (input.value.length === CODE_LENGTH) check();
  });

  // Checking never joins: a returning player who mistypes into another team's code stays on their own.
  async function check() {
    if (input.value.length < CODE_LENGTH) { input.focus(); return; }
    if (go.dataset.btn === 'checking' || go.dataset.btn === 'error') return;
    const code = input.value;
    setButton('checking');
    let found = false, failed = false;
    try {
      const [res] = await Promise.all([
        fetch('/play/check', {
          method: 'POST',
          headers: { 'X-CSRF-Token': form.elements.csrf.value, 'Content-Type': 'application/x-www-form-urlencoded' },
          body: new URLSearchParams({ run: code }),
        }),
        // Stops a fast answer flickering past "Checking".
        new Promise(r => setTimeout(r, reduced ? 0 : 450)),
      ]);
      found = res.ok && (await res.json()).found;
      failed = !res.ok && res.status !== 404;
    } catch {
      failed = true;
    }
    if (input.value !== code) return;
    if (!found) {
      showError(failed ? 'Something went wrong. Try again.' : 'Check the game code with your facilitator.');
      input.select();
      return;
    }
    setButton('ready');
    go.removeAttribute('data-pop'); void go.offsetWidth; go.setAttribute('data-pop', '');
    burst(go, 22, 14, 19);
    stickerEls.forEach((b, i) => setTimeout(() => hop(b), i * 60));
  }
  form.addEventListener('submit', e => {
    e.preventDefault();
    if (go.dataset.btn === 'ready') {
      go.setAttribute('data-go', '');
      // Waits out the arrow animation. form.submit() skips this handler.
      setTimeout(() => form.submit(), reduced ? 0 : 380);
      return;
    }
    check();
  });

  // Each leg starts at the card it leaves so no drawing time is spent hidden behind one.
  const below = $('below');
  let belowPlayed = false, belowTotal = 0;
  function drawBelowTrail() {
    const w = below.clientWidth, h = below.clientHeight, br = below.getBoundingClientRect();
    const box = k => {
      const r = below.querySelector(`[data-node="${k}"]`).getBoundingClientRect();
      return { x: r.left - br.left + r.width / 2, top: r.top - br.top, bottom: r.bottom - br.top };
    };
    const [a, b, c, dn] = ['a', 'b', 'c', 'd'].map(box);
    // Vertical tangents at both ends so no leg kinks at a card.
    const leg = (p, q) => {
      const k = (q[1] - p[1]) * .95;
      return `M${p[0].toFixed(1)} ${p[1].toFixed(1)} C${p[0].toFixed(1)} ${(p[1] + k).toFixed(1)} ${q[0].toFixed(1)} ${(q[1] - k).toFixed(1)} ${q[0].toFixed(1)} ${q[1].toFixed(1)}`;
    };
    const t = $('trail2');
    t.setAttribute('viewBox', `0 0 ${w} ${h}`);
    t.innerHTML = masked(below, [
      leg([a.x, a.bottom], [b.x, b.top]),
      leg([a.x, a.bottom], [c.x, c.top]),
      leg([b.x, b.bottom], [dn.x, dn.top]),
      leg([c.x, c.bottom], [dn.x, dn.top]),
    ], 'rvb');
    if (belowPlayed || reduced) { t.setAttribute('data-done', ''); below.setAttribute('data-play', ''); return 0; }
    // Two waves, fork then join; each card unlocks as its wave lands.
    const SPEED = 600, rvs = t.querySelectorAll('.home-rv');
    const inks = [...t.querySelectorAll('.home-ink')].map(p => p.getTotalLength() / SPEED);
    let start = .6;
    const lands = [[0, 1], [2, 3]].map(ws => {
      const dur = Math.max(...ws.map(i => inks[i]));
      ws.forEach(i => { rvs[i].style.setProperty('--dur', inks[i] + 's'); rvs[i].style.animationDelay = start + 's'; });
      return (start += dur);
    });
    [['a', 0], ['b', lands[0]], ['c', lands[0]], ['d', lands[1]]].forEach(([k, s]) => {
      const n = below.querySelector(`[data-node="${k}"]`);
      n.style.setProperty('--at', s.toFixed(2) + 's');
      n.querySelector('.home-mark').style.setProperty('--at', s.toFixed(2) + 's');
    });
    return lands[1];
  }
  observers.push(new IntersectionObserver((es, o) => {
    if (!es[0].isIntersecting) return;
    o.disconnect();
    const t = $('trail2');
    t.setAttribute('data-play', '');
    below.setAttribute('data-play', '');
    // A resize mid-draw must not restart the animation, so only mark played once it finishes.
    setTimeout(() => { belowPlayed = true; settle(t); }, belowTotal * 1000 + 400);
  }, { threshold: .25 }));
  observers.at(-1).observe(below);

  let ht, bt;
  observers.push(new ResizeObserver(() => { clearTimeout(ht); ht = setTimeout(drawTrail, 60); }), new ResizeObserver(() => { clearTimeout(bt); bt = setTimeout(() => { belowTotal = drawBelowTrail() || belowTotal; }, 60); }));
  observers.at(-2).observe(hero);
  observers.at(-1).observe(below);
  window.siteLeave?.(() => { clearTimeout(ht); clearTimeout(bt); });
  render();
  joining();
})();
