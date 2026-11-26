(() => {
  const $ = id => document.getElementById(id);
  const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
  const ac = new AbortController();
  let gone = false;
  window.siteLeave?.(() => { gone = true; ac.abort(); });
  const wait = ms => new Promise(res => setTimeout(res, ms));

  // Loops wait at the gate between steps so motion never runs unwatched.
  const gates = [];
  const isOpen = g => !g.paused && g.seen && !document.hidden;
  const release = g => {
    g.el.toggleAttribute('data-still', !isOpen(g));
    if (isOpen(g) || gone) g.waiters.splice(0).forEach(f => f());
  };
  const until = g => isOpen(g) || gone ? Promise.resolve() : new Promise(res => g.waiters.push(res));
  const io = new IntersectionObserver(es => es.forEach(e => {
    const g = gates.find(x => x.el === e.target);
    if (g) { g.seen = e.isIntersecting; release(g); }
  }));
  document.addEventListener('visibilitychange', () => gates.forEach(release), { signal: ac.signal });
  window.siteLeave?.(() => { io.disconnect(); gates.forEach(release); });
  const PAUSE = '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="6" y="5" width="4" height="14" rx="1"/><rect x="14" y="5" width="4" height="14" rx="1"/></svg>';
  const PLAY = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 4.5v15a.5.5 0 0 0 .77.42l11.5-7.5a.5.5 0 0 0 0-.84L7.77 4.08A.5.5 0 0 0 7 4.5z"/></svg>';
  function gate(el, paused, onPlay) {
    const g = { el, paused, seen: true, waiters: [] };
    gates.push(g); io.observe(el); release(g);
    const b = document.createElement('button');
    b.type = 'button'; b.className = 'cr-pause';
    const label = () => { b.innerHTML = (g.paused ? PLAY + 'Play' : PAUSE + 'Pause') + '<span class="sr-only"> the demo</span>'; };
    b.addEventListener('click', () => { g.paused = !g.paused; label(); release(g); if (!g.paused) onPlay?.(); });
    label(); el.after(b);
    return g;
  }
  const SVG = d => `<svg viewBox="0 0 24 24" aria-hidden="true">${d}</svg>`;
  const ICONS = {
    flag: '<path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z"/><path d="M4 22v-7"/>',
    pin: '<path d="M20 10c0 4.993-5.539 10.193-7.399 11.799a1 1 0 0 1-1.202 0C9.539 20.193 4 14.993 4 10a8 8 0 0 1 16 0"/><circle cx="12" cy="10" r="3"/>',
    user: '<circle cx="12" cy="8" r="5"/><path d="M20 21a8 8 0 0 0-16 0"/>',
    users: '<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/>',
    camera: '<path d="M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3l-2.5-3z"/><circle cx="12" cy="13" r="3"/>',
    quiz: '<circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><path d="M12 17h.01"/>',
    choice: '<circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="3" fill="currentColor"/>',
    keypad: '<path d="M5 5h.01M12 5h.01M19 5h.01M5 12h.01M12 12h.01M19 12h.01M5 19h.01M12 19h.01M19 19h.01" stroke-width="3.4"/>',
    asterisk: '<path d="M12 6v12"/><path d="M17.196 9 6.804 15"/><path d="m6.804 9 10.392 6"/>',
    sort: '<path d="m3 16 4 4 4-4"/><path d="M7 20V4"/><path d="m21 8-4-4-4 4"/><path d="M17 4v16"/>',
    list: '<path d="m3 17 2 2 4-4"/><path d="m3 7 2 2 4-4"/><path d="M13 6h8"/><path d="M13 12h8"/><path d="M13 18h8"/>',
    pen: '<path d="M12 20h9"/><path d="M16.376 3.622a1 1 0 0 1 3.002 3.002L7.368 18.635a2 2 0 0 1-.855.506l-2.872.838a.5.5 0 0 1-.62-.62l.838-2.872a2 2 0 0 1 .506-.854z"/>',
    qr: '<path d="M17 12v4a1 1 0 0 1-1 1h-4"/><path d="M17 3h2a2 2 0 0 1 2 2v2"/><path d="M17 8V7"/><path d="M21 17v2a2 2 0 0 1-2 2h-2"/><path d="M3 7V5a2 2 0 0 1 2-2h2"/><path d="M7 17h.01"/><path d="M7 21H5a2 2 0 0 1-2-2v-2"/><rect x="7" y="7" width="5" height="5" rx="1"/>',
    star: '<path d="M11.525 2.295a.53.53 0 0 1 .95 0l2.31 4.679a2.123 2.123 0 0 0 1.595 1.16l5.166.756a.53.53 0 0 1 .294.904l-3.736 3.638a2.123 2.123 0 0 0-.611 1.878l.882 5.14a.53.53 0 0 1-.771.56l-4.618-2.428a2.122 2.122 0 0 0-1.973 0L6.396 21.01a.53.53 0 0 1-.77-.56l.881-5.139a2.122 2.122 0 0 0-.611-1.879L2.16 9.795a.53.53 0 0 1 .294-.906l5.165-.755a2.122 2.122 0 0 0 1.597-1.16z"/>',
    text: '<path d="M17 6.1H3"/><path d="M21 12.1H3"/><path d="M15.1 18H3"/>',
    heading: '<path d="M6 12h12"/><path d="M6 20V4"/><path d="M18 20V4"/>',
    image: '<rect width="18" height="18" x="3" y="3" rx="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.086-3.086a2 2 0 0 0-2.828 0L6 21"/>',
    video: '<path d="M2.5 17a24.12 24.12 0 0 1 0-10 2 2 0 0 1 1.4-1.4 49.56 49.56 0 0 1 16.2 0A2 2 0 0 1 21.5 7a24.12 24.12 0 0 1 0 10 2 2 0 0 1-1.4 1.4 49.55 49.55 0 0 1-16.2 0A2 2 0 0 1 2.5 17"/><path d="m10 15 5-3-5-3z"/>',
    bulb: '<path d="M15 14c.2-1 .7-1.7 1.5-2.5 1-.9 1.5-2.2 1.5-3.5A6 6 0 0 0 6 8c0 1 .2 2.2 1.5 3.5.7.7 1.3 1.5 1.5 2.5"/><path d="M9 18h6"/><path d="M10 22h4"/>',
    eye: '<path d="M2.062 12.348a1 1 0 0 1 0-.696 10.75 10.75 0 0 1 19.876 0 1 1 0 0 1 0 .696 10.75 10.75 0 0 1-19.876 0"/><circle cx="12" cy="12" r="3"/>',
    dice: '<rect width="18" height="18" x="3" y="3" rx="2"/><path d="M16 8h.01M8 8h.01M8 16h.01M16 16h.01M12 12h.01" stroke-width="3"/>',
    map: '<path d="M14.106 5.553a2 2 0 0 0 1.788 0l3.659-1.83A1 1 0 0 1 21 4.619v12.764a1 1 0 0 1-.553.894l-4.553 2.277a2 2 0 0 1-1.788 0l-4.212-2.106a2 2 0 0 0-1.788 0l-3.659 1.83A1 1 0 0 1 3 19.381V6.618a1 1 0 0 1 .553-.894l4.553-2.277a2 2 0 0 1 1.788 0z"/><path d="M15 5.764v15"/><path d="M9 3.236v15"/>',
    alert: '<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/><path d="M12 9v4"/><path d="M12 17h.01"/>',
    link: '<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>',
  };
  // Matches the objective editor's rule icons.
  const RULE_ICONS = {
    guided: '<path d="M12 13v8"/><path d="M12 3v3"/><path d="M4 6a1 1 0 0 0-1 1v5a1 1 0 0 0 1 1h13a2 2 0 0 0 1.152-.365l3.424-2.317a1 1 0 0 0 0-1.635l-3.424-2.318A2 2 0 0 0 17 6z"/>',
    open: '<path d="M4 16v-2.38C4 11.5 2.97 10.5 3 8c.03-2.72 1.49-6 4.5-6C9.37 2 10 3.8 10 5.5c0 3.11-2 5.66-2 8.68V16a2 2 0 1 1-4 0Z"/><path d="M20 20v-2.38c0-2.12 1.03-3.12 1-5.62-.03-2.72-1.49-6-4.5-6C14.63 6 14 7.8 14 9.5c0 3.11 2 5.66 2 8.68V20a2 2 0 1 0 4 0Z"/><path d="M16 17h4"/><path d="M4 13h4"/>',
    random: '<path d="m18 14 4 4-4 4"/><path d="m18 2 4 4-4 4"/><path d="M2 18h1.973a4 4 0 0 0 3.3-1.7l5.454-8.6a4 4 0 0 1 3.3-1.7H22"/><path d="M2 6h1.972a4 4 0 0 1 3.6 2.2"/><path d="M22 18h-6.041a4 4 0 0 1-3.3-1.8l-.359-.45"/>',
    nest: '<path d="M8 5h13"/><path d="M13 12h8"/><path d="M13 19h8"/><path d="M3 10a2 2 0 0 0 2 2h3"/><path d="M3 5v12a2 2 0 0 0 2 2h3"/>',
    pass: '<path d="M9 10h.01"/><path d="M15 10h.01"/><path d="M12 2a8 8 0 0 0-8 8v12l3-3 2.5 2.5L12 19l2.5 2.5L17 19l3 3V10a8 8 0 0 0-8-8z"/>',
    gate: '<rect width="18" height="11" x="3" y="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>',
  };
  const TICK = SVG('<path d="M20 6 9 17l-5-5"/>');
  const LOCK = SVG(RULE_ICONS.gate);
  const UNLOCK = SVG('<rect width="18" height="11" x="3" y="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 9.9-1"/>');
  const KEY = SVG('<path d="M2.586 17.414A2 2 0 0 0 2 18.828V21a1 1 0 0 0 1 1h3a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h1a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h.172a2 2 0 0 0 1.414-.586l.814-.814a6.5 6.5 0 1 0-4-4z"/><circle cx="16.5" cy="7.5" r=".5" fill="currentColor"/>');
  document.querySelectorAll('.st[data-ic]').forEach(el => el.insertAdjacentHTML('afterbegin', SVG(ICONS[el.dataset.ic])));
  const shuffle = a => a.map(x => [Math.random(), x]).sort((p, q) => p[0] - q[0]).map(p => p[1]);

  const PROVE = [['qr', 'Scan a code'], ['pin', 'Stand on the spot'], ['camera', 'Snap a photo'], ['quiz', 'Quiz'], ['choice', 'Pick an option'], ['pen', 'Write an answer'], ['asterisk', 'Password'], ['keypad', 'Pin code'], ['sort', 'Put in order'], ['list', 'Checklist'], ['star', 'Rate it'], ['users', 'Name your team']];
  const TELL = [['text', 'Text'], ['heading', 'Heading'], ['image', 'Image'], ['video', 'Video'], ['bulb', 'Clue'], ['eye', 'Tap to reveal'], ['dice', 'Different for each team'], ['map', 'Map'], ['alert', 'Callout'], ['link', 'Button link']];
  const chip = ([ic, name], i) => `<li data-k="${ic}"><span class="ic" style="--r:${i % 2 ? 5 : -5}deg">${SVG(ICONS[ic])}</span>${name}</li>`;
  $('prove').innerHTML = PROVE.map(chip).join('');
  $('tell').innerHTML = TELL.map(chip).join('');

  // Decorative: the section is plain ground if this fails.
  fetch('/static/js/maps/wellington.json').then(r => r.json()).then(m => {
    for (const k of ['water', 'river', 'coast', 'street', 'major']) $('s-' + k).setAttribute('d', m[k] || '');
  }).catch(() => {});

  (() => {
    const stack = $('stack');
    const rule = (icon, words) => `<span class="rule"><i>${icon}</i>${words}</span>`;
    const STEPS = [
      ['text', 'story', 'Text', '<p>He\'s been keeping a secret for 120 years. Find him by the library steps.</p>'],
      ['image', 'story', 'Image', '<div class="pic"></div>'],
      ['quiz', 'ordered', 'Quiz', `<h4>What's he glaring at?</h4><div class="opts"><span>The clock</span><span>A pigeon</span><span>The café</span></div>${rule(KEY, 'Hands out the map key')}`],
      ['bulb', 'story', 'Clue', `<p class="hid">The other half of the map is with his missing hat.</p>${rule(LOCK, 'Waits for the map key')}`],
    ];
    const block = ([ic, r, name, body]) => `<li class="blk" data-r="${r}"><div class="blk-t">${SVG(ICONS[ic])}${name}</div>${body}</li>`;
    // A hidden copy of the finished stack shares the grid cell so the card holds its final height
    // without measuring, which forced a whole-page layout during load.
    const wrap = document.createElement('div');
    wrap.className = 'stack-wrap';
    stack.before(wrap);
    wrap.innerHTML = `<ol class="stack stack-ghost" aria-hidden="true">${STEPS.map(block).join('')}</ol>`;
    wrap.append(stack);
    if (reduced) { stack.innerHTML = STEPS.map(block).join(''); return; }
    const bg = gate(stack.closest('.stopcard'), false);
    const step = async ms => { await wait(ms); await until(bg); };
    const pick = k => document.querySelector(`.chips li[data-k="${k}"]`);
    const loop = async () => {
      stack.innerHTML = '';
      await step(400);
      for (const s of STEPS) {
        if (gone) return;
        const c = pick(s[0]);
        c.setAttribute('data-pick', '');
        await step(550);
        c.removeAttribute('data-pick');
        stack.insertAdjacentHTML('beforeend', block(s));
        await step(900);
      }
      await step(3000);
      if (!gone) loop();
    };
    loop();
  })();

  const STOPS = {
    P: { ic: 'user', r: 0, l: 'Your team', me: true },
    A: { ic: 'qr', r: -6, l: 'The grumpy statue' }, B: { ic: 'keypad', r: 5, l: 'The captain\'s chest' },
    C: { ic: 'camera', r: -4, l: 'The gargoyle' }, D: { ic: 'quiz', r: 6, l: 'Lions at the library' },
    E: { ic: 'flag', r: -5, l: 'The secret garden' }, F: { ic: 'sort', r: 4, l: 'The fossil drawer' },
    G: { ic: 'pin', r: -5, l: 'The old pier' },
    c1: { ic: 'star', r: 6, l: 'Bonus: the bell' }, c2: { ic: 'star', r: -5, l: 'Bonus: the mural' }, c3: { ic: 'star', r: 4, l: 'Bonus: the fountain' },
  };

  function makeMap(stage, sc) {
    stage.removeAttribute('data-path');
    stage.innerHTML = '<svg class="topo"></svg><svg class="trails" viewBox="0 0 100 100" preserveAspectRatio="none"></svg>';
    const trails = stage.querySelector('.trails'), nodes = {}, spots = { ...sc.spots }, links = {};
    if (sc.room) stage.insertAdjacentHTML('beforeend', `<div class="room" ${sc.room.ghost ? 'data-ghost' : ''} data-r="${sc.room.r}" style="--x:${sc.room.x};--y:${sc.room.y};--w:${sc.room.w};--h:${sc.room.h}"><span>${sc.room.label}</span></div>`);
    if (sc.view) stage.insertAdjacentHTML('beforeend', '<span class="view"></span>');
    for (const [id, p] of Object.entries(spots)) {
      const s = STOPS[id], el = document.createElement('div');
      el.className = 'nd' + (s.me ? ' me' : '');
      el.dataset.r = sc.hues[id] || sc.hues['*'];
      el.style.cssText = `--x:${p[0]};--y:${p[1]};--r:${s.r}deg`;
      el.innerHTML = `<div class="nd-b">${SVG(ICONS[s.ic])}<b>?</b></div><span class="mk tick">${TICK}</span>${sc.labels ? `<span class="nd-l">${s.l}</span>` : ''}`;
      stage.appendChild(el);
      nodes[id] = el;
    }
    window.siteTopo?.(stage.querySelector('.topo'), true);
    const pt = id => Array.isArray(id) ? id : spots[id];
    const curve = (a, b) => {
      const [x1, y1] = pt(a), [x2, y2] = pt(b), k = .14;
      const cx = (x1 + x2) / 2 - (y2 - y1) * k, cy = (y1 + y2) / 2 + (x2 - x1) * k;
      return `M${x1} ${y1}Q${cx.toFixed(1)} ${cy.toFixed(1)} ${x2} ${y2}`;
    };
    const key = (a, b) => [a, b].sort().join('-');
    const m = {
      stage, nodes, pt, room: stage.querySelector('.room'), view: stage.querySelector('.view'),
      set: (id, s) => { nodes[id].dataset.s = s; },
      state: id => nodes[id].dataset.s,
      move(id, p) { spots[id] = p; nodes[id].style.setProperty('--x', p[0]); nodes[id].style.setProperty('--y', p[1]); },
      pop: id => { if (reduced) return; const el = nodes[id]; el.removeAttribute('data-pop'); void el.offsetWidth; el.setAttribute('data-pop', ''); },
      seg(a, b) { m.unlink(a, b); trails.insertAdjacentHTML('beforeend', `<path class="seg" d="${curve(a, b)}"/>`); },
      link(a, b) { const k = key(a, b); if (links[k]) return; trails.insertAdjacentHTML('afterbegin', `<path class="link" d="${curve(a, b)}"/>`); links[k] = trails.firstElementChild; },
      unlink(a, b) { const k = key(a, b), el = links[k]; if (!el) return; el.setAttribute('data-gone', ''); delete links[k]; setTimeout(() => el.remove(), 500); },
      unlinkAll(id) { Object.keys(links).forEach(k => { if (k.split('-').includes(id)) m.unlink(...k.split('-')); }); },
      clear() { trails.innerHTML = ''; Object.keys(links).forEach(k => delete links[k]); stage.querySelectorAll('.fly').forEach(e => e.remove()); m.room?.removeAttribute('data-s'); Object.assign(spots, sc.spots); Object.entries(sc.spots).forEach(([id, p]) => m.move(id, p)); },
      reveal: () => stage.querySelector('.trails'),
    };
    return m;
  }

  let run = 0, still = reduced;
  const beat = async (r, ms) => {
    await wait(still ? 0 : ms);
    if (!still) await until(stageGate);
    return r === run && !gone;
  };
  // A still scene stops here, on its finished state.
  const hold = (r, ms) => still ? Promise.resolve(false) : beat(r, ms);

  const BLOCKS = [
    {
      key: 'guided', name: 'Guided path', r: 'ordered',
      what: 'One stop at a time, in the order you set. The whole path is there from the start; the next stop lights up only once they\'ve cleared this one.',
      ideas: ['A mystery where clue two only makes sense after clue one', 'An orientation walk: library, then student centre, then the café', 'A story told in chapters around a gallery'],
      spots: { P: [50, 9], A: [27, 25], B: [72, 39], C: [30, 55], D: [70, 71], E: [46, 88] }, hues: { '*': 'ordered' },
      async play(m, r) {
        const order = ['P', 'A', 'B', 'C', 'D', 'E'];
        for (;;) {
          m.clear();
          order.forEach((id, i) => { if (i) m.link(order[i - 1], id); m.set(id, i === 1 ? 'live' : i ? 'idle' : 'done'); });
          if (!await beat(r, 1100)) return;
          for (let i = 1; i < order.length; i++) {
            m.seg(order[i - 1], order[i]); m.set(order[i], 'done'); m.pop(order[i]);
            if (i + 1 < order.length) m.set(order[i + 1], 'live');
            if (!await beat(r, 1000)) return;
          }
          if (!await hold(r, 1800)) return;
        }
      },
    },
    {
      key: 'open', name: 'Open exploration', r: 'free_roam',
      what: 'Everything at once, any order. Every stop links to every other; as a team moves on, the routes they can no longer take fall away and only the walk they chose is left.',
      ideas: ['A gallery floor teams can wander', 'An O-week hunt across the whole campus', 'Side quests anyone can grab between the main stops'],
      spots: { P: [50, 10], A: [18, 34], B: [80, 30], C: [50, 52], D: [22, 80], E: [78, 78] }, hues: { '*': 'free_roam' },
      async play(m, r) {
        const all = ['P', 'A', 'B', 'C', 'D', 'E'];
        for (;;) {
          m.clear();
          all.forEach((a, i) => all.slice(i + 1).forEach(b => m.link(a, b)));
          all.forEach(id => m.set(id, id === 'P' ? 'done' : 'live'));
          if (!await beat(r, 1500)) return;
          let at = 'P';
          for (const id of shuffle(all.slice(1))) {
            m.seg(at, id); m.unlinkAll(at); at = id;
            m.set(id, 'done'); m.pop(id);
            if (!await beat(r, 1100)) return;
          }
          m.unlinkAll(at);
          if (!await hold(r, 1800)) return;
        }
      },
    },
    {
      key: 'random', name: 'Randomised', r: 'randomised',
      what: 'The deck is shuffled for each team and a few are dealt. Different teams get different hands, so they spread out instead of queuing.',
      ideas: ['Thirty students, twelve statues: spread the crowd', 'Replays that never go the same way twice', 'Stop teams copying each other\'s answers'],
      spots: { P: [50, 10], A: [20, 38], B: [50, 38], C: [80, 38], D: [20, 62], E: [50, 62], F: [80, 62], G: [50, 85] }, hues: { '*': 'randomised' },
      async play(m, r) {
        const deck = ['A', 'B', 'C', 'D', 'E', 'F', 'G'];
        const slots = deck.map(id => m.pt(id));
        for (;;) {
          m.clear(); m.set('P', 'done');
          deck.forEach(id => m.set(id, 'down'));
          if (!await beat(r, 900)) return;
          let order = deck;
          for (let k = 0; k < 3; k++) {
            order = shuffle(order);
            order.forEach((id, i) => m.move(id, slots[i]));
            if (!await beat(r, 650)) return;
          }
          const hand = order.slice(0, 3);
          for (const id of hand) { m.set(id, 'live'); m.pop(id); m.link('P', id); if (!await beat(r, 350)) return; }
          if (!await beat(r, 1100)) return;
          let at = 'P';
          for (const id of shuffle(hand)) {
            m.seg(at, id); m.unlinkAll(at); at = id;
            m.set(id, 'done'); m.pop(id);
            hand.filter(h => m.state(h) === 'live').forEach(h => m.link(at, h));
            if (!await beat(r, 1100)) return;
          }
          if (!await hold(r, 1800)) return;
        }
      },
    },
    {
      key: 'nest', name: 'Nesting', r: 'free_roam',
      what: 'Any stop can be a room with its own rules inside. A guided tour can open onto a gallery teams roam, and move on once they\'ve done enough of it.',
      ideas: ['Each floor of a museum as its own room', 'A guided tour with free time in the middle', 'A puzzle box inside a room inside a quest'],
      spots: { P: [12, 90], A: [16, 68], B: [24, 44], C: [54, 62], c2: [66, 80], D: [77, 62], E: [44, 15] },
      hues: { '*': 'ordered', C: 'free_roam', c2: 'free_roam', D: 'free_roam' },
      room: { x: 42, y: 50, w: 46, h: 42, r: 'free_roam', label: 'Gallery · any 2 of 3' },
      async play(m, r) {
        const kids = ['C', 'c2', 'D'];
        for (;;) {
          m.clear(); m.set('P', 'done'); ['B', 'E', ...kids].forEach(id => m.set(id, 'idle')); m.set('A', 'live');
          if (!await beat(r, 1000)) return;
          m.seg('P', 'A'); m.set('A', 'done'); m.pop('A'); m.set('B', 'live');
          if (!await beat(r, 1000)) return;
          m.seg('A', 'B'); m.set('B', 'done'); m.pop('B');
          kids.forEach(id => { m.set(id, 'live'); m.pop(id); });
          if (!await beat(r, 1100)) return;
          let at = 'B';
          for (const id of shuffle(kids).slice(0, 2)) { m.seg(at, id); at = id; m.set(id, 'done'); m.pop(id); if (!await beat(r, 900)) return; }
          kids.forEach(id => { if (m.state(id) !== 'done') m.set(id, 'idle'); });
          m.room.dataset.s = 'done'; m.set('E', 'live');
          if (!await beat(r, 1000)) return;
          m.seg(at, 'E'); m.set('E', 'done'); m.pop('E');
          if (!await hold(r, 1800)) return;
        }
      },
    },
    {
      key: 'pass', name: 'Pass-through', r: 'randomised',
      what: 'A group players never see. It still routes, counts and locks, but its stops fold into the path around it, with no extra screen to open.',
      ideas: ['Deal one of three bonus stops without a separate section', 'Put one lock on several stops at once', 'Require any two of a set inside an otherwise guided path'],
      spots: { P: [13, 8], A: [13, 21], B: [13, 34], c1: [13, 55], c2: [13, 69], c3: [13, 83] }, labels: true, view: true,
      hues: { '*': 'ordered', c1: 'randomised', c2: 'randomised', c3: 'randomised' },
      room: { x: 5, y: 45, w: 88, h: 47, r: 'randomised', label: 'Pass-through · deal 1 of 3', ghost: true },
      async play(m, r) {
        const bonus = ['c1', 'c2', 'c3'];
        for (;;) {
          m.clear(); m.stage.removeAttribute('data-path'); m.room.removeAttribute('data-hide');
          m.view.textContent = 'What you build';
          m.set('P', 'done'); ['A', 'B'].forEach(id => m.set(id, 'idle')); bonus.forEach(id => m.set(id, 'idle'));
          if (!await beat(r, 1300)) return;
          const pick = bonus[Math.floor(Math.random() * 3)];
          for (let k = 0; k < 7; k++) {
            const id = bonus[k % 3];
            m.nodes[id].setAttribute('data-flash', '');
            if (!await beat(r, 160)) return;
            m.nodes[id].removeAttribute('data-flash');
          }
          bonus.forEach(id => m.set(id, id === pick ? 'live' : 'down')); m.pop(pick);
          if (!await beat(r, 1600)) return;
          m.view.textContent = 'What a team sees';
          m.room.setAttribute('data-hide', '');
          bonus.forEach(id => { if (id !== pick) m.set(id, 'gone'); });
          if (!await beat(r, 500)) return;
          m.stage.setAttribute('data-path', '');
          const path = ['P', 'A', 'B', pick], at = [[50, 12], [28, 38], [70, 58], [42, 84]];
          path.forEach((id, i) => m.move(id, at[i]));
          if (!await beat(r, 650)) return;
          path.forEach((id, i) => { if (i) m.link(path[i - 1], id); });
          m.set('A', 'live');
          if (!await beat(r, 900)) return;
          for (let i = 1; i < path.length; i++) {
            m.seg(path[i - 1], path[i]); m.set(path[i], 'done'); m.pop(path[i]);
            if (i + 1 < path.length) m.set(path[i + 1], 'live');
            if (!await beat(r, 950)) return;
          }
          if (!await hold(r, 1600)) return;
        }
      },
    },
    {
      key: 'gate', name: 'Gating', r: 'keys',
      what: 'Answers hand out keys. A stop, or a paragraph, photo or clue inside one, waits until a team holds every key it names. Teams who found more read more.',
      ideas: ['Pick a side early, read a different ending', 'The final door needs a key from every floor', 'Show a hint only to teams who found the map'],
      spots: { P: [50, 12], B: [24, 38], D: [76, 38], E: [50, 68] }, hues: { '*': 'free_roam' },
      async play(m, r) {
        const E = m.nodes.E;
        E.insertAdjacentHTML('beforeend', `<span class="needs"><i class="slot">${LOCK}</i><i class="slot">${LOCK}</i></span>`);
        ['B', 'D'].forEach(id => m.nodes[id].insertAdjacentHTML('beforeend', `<span class="kc">${KEY}</span>`));
        const needs = E.querySelector('.needs'), locks = [...needs.children];
        // Measured, not derived from the spots, so a key lands on its padlock at any stage size.
        const centre = el => {
          const s = m.stage.getBoundingClientRect(), b = el.getBoundingClientRect();
          return [(b.left + b.width / 2 - s.left) / s.width * 100, (b.top + b.height / 2 - s.top) / s.height * 100];
        };
        const fly = async (from, lock) => {
          const f = document.createElement('div'), to = centre(lock);
          f.className = 'fly'; f.innerHTML = KEY;
          f.style.cssText = `--x:${from[0]};--y:${from[1]}`;
          m.stage.appendChild(f); void f.offsetWidth;
          f.style.cssText = `--x:${to[0]};--y:${to[1]}`;
          if (!await beat(r, 750)) return false;
          f.remove();
          return true;
        };
        for (;;) {
          m.clear();
          locks.forEach(l => { l.removeAttribute('data-held'); l.innerHTML = LOCK; }); needs.style.opacity = 1;
          m.set('P', 'done'); m.set('B', 'live'); m.set('D', 'live'); m.set('E', 'idle');
          if (!await beat(r, 1200)) return;
          let at = 'P';
          for (const [i, id] of ['B', 'D'].entries()) {
            m.seg(at, id); at = id;
            // Read before set() fades the corner out with the done stop.
            const from = centre(m.nodes[id].querySelector('.kc'));
            m.set(id, 'done'); m.pop(id);
            if (!await fly(from, locks[i])) return;
            locks[i].setAttribute('data-held', ''); locks[i].innerHTML = UNLOCK;
            if (!await beat(r, 600)) return;
          }
          m.set('E', 'live'); m.pop('E');
          if (!await beat(r, 700)) return;
          needs.style.opacity = 0;
          if (!await beat(r, 500)) return;
          m.seg(at, 'E'); m.set('E', 'done'); m.pop('E');
          if (!await hold(r, 2000)) return;
        }
      },
    },
  ];

  document.querySelectorAll('.sic[data-ic]').forEach(el => el.innerHTML = SVG(ICONS[el.dataset.ic]));

  (() => {
    const list = $('hist');
    if (!list) return;
    const row = list.querySelector('[data-target]'), btn = row.querySelector('.undo');
    // Pinned so the undo row cannot move the page; re-measured when a width change rewraps.
    const pin = () => { const fresh = list.querySelector('.fresh'); fresh?.remove(); list.style.height = ''; list.style.height = list.offsetHeight + 'px'; if (fresh) list.prepend(fresh); };
    pin();
    let w = innerWidth;
    addEventListener('resize', () => { if (innerWidth !== w) { w = innerWidth; pin(); } }, { signal: ac.signal });
    const undo = () => {
      btn.removeAttribute('data-press'); row.setAttribute('data-undone', ''); btn.textContent = 'Undone';
      list.insertAdjacentHTML('afterbegin', '<li class="fresh"><span class="what">Undid: changed the quiz answer on <b>The gargoyle</b></span><time>Just now</time></li>');
      list.setAttribute('data-full', '');
    };
    if (reduced) { undo(); return; }
    const hg = gate(list.closest('.shot'), false);
    const loop = async () => {
      if (gone) return;
      list.querySelector('.fresh')?.remove(); list.removeAttribute('data-full'); row.removeAttribute('data-undone'); btn.textContent = 'Undo';
      await wait(1800); await until(hg);
      if (gone) return;
      btn.setAttribute('data-press', '');
      await wait(300);
      undo();
      await wait(3900); await until(hg);
      loop();
    };
    loop();
  })();

  const tabs = $('tabs'), about = $('about'), stage = $('stage');
  let current = 0;
  const stageGate = gate(stage, reduced, () => { if (still) { still = false; show(current); } });
  BLOCKS.forEach((b, i) => tabs.insertAdjacentHTML('beforeend', `<button type="button" class="tab" role="tab" aria-controls="panel" id="tab-${b.key}" data-r="${b.r}" style="--r:${i % 2 ? 2 : -2}deg" aria-selected="false" tabindex="-1">${SVG(RULE_ICONS[b.key])}${b.name}</button>`));
  const tabEls = [...tabs.children];
  function show(i) {
    current = i;
    const b = BLOCKS[i];
    tabEls.forEach((t, j) => { t.setAttribute('aria-selected', i === j); t.tabIndex = i === j ? 0 : -1; });
    $('panel').setAttribute('aria-labelledby', 'tab-' + b.key);
    about.dataset.r = b.r;
    about.innerHTML = `<h3>${b.name}</h3><p>${b.what}</p><h4>Try it for</h4><ul class="ideas">${b.ideas.map(x => `<li>${x}</li>`).join('')}</ul>`;
    const r = ++run;
    b.play(makeMap(stage, b), r);
  }
  tabEls.forEach((t, i) => t.addEventListener('click', () => show(i)));
  tabs.addEventListener('keydown', e => {
    const i = tabEls.indexOf(document.activeElement);
    if (i < 0) return;
    const last = tabEls.length - 1;
    const n = { ArrowRight: i === last ? 0 : i + 1, ArrowLeft: i ? i - 1 : last, Home: 0, End: last }[e.key];
    if (n === undefined) return;
    e.preventDefault();
    tabEls[n].focus(); show(n);
  });


  const q = new URLSearchParams(location.search);
  show(Math.max(0, BLOCKS.findIndex(b => b.key === q.get('tab'))));
})();
