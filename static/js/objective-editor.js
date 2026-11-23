// Every block's form stays in the page, hidden until selected, so an edit still
// saving when the author picks another block is not lost with its form.
//
// Re-runs on each boosted navigation: document listeners bind once, init()
// rebuilds the per-page parts.
(() => {
	const q = (selector, root = document) => root.querySelector(selector);
	const qa = (selector, root = document) => [...root.querySelectorAll(selector)];

	const state = (window.__objectiveEditor = window.__objectiveEditor || { bound: false, selected: {} });
	const ORDER_PATH = /\/admin\/blocks\/(reorder|[^/]+\/move)$/;
	// Every request that changes the list: adding (alone or as a template), then ordering.
	const LIST_PATH = /^\/admin\/blocks\/(kit|reorder|[^/]+\/move)?$/;
	// Long enough to read the message before the page resyncs.
	const RELOAD_DELAY = 1800;

	// A page has one section (a system page) or several (an objective's proof and reveal).
	// The task has a section too, but no blocks to edit.
	const zones = () => qa('[data-sec]').filter((sec) => q('[data-rail]', sec)).map((sec) => sec.dataset.sec);
	const section = (zone) => q(`[data-sec="${zone}"]`);
	const rail = (zone) => q(`[data-rail="${zone}"]`);
	const railItem = (id) => q(`.rl-item[data-id="${id}"]`);
	const blockForm = (id) => q(`[data-block-form="${id}"]`);
	const phoneBlock = (id) => q(`#preview-block-${id}`);
	const idsIn = (zone) => (rail(zone) ? qa('.rl-item', rail(zone)).map((item) => item.dataset.id) : []);
	const zoneOfItem = (item) => item.closest('[data-rail]').dataset.rail;
	const pointsEnabled = () => q('[data-objective-editor]')?.dataset.pointsEnabled === 'true';
	const locked = () => document.body.dataset.questLocked === 'true';
	const phoneList = (zone) => q('[data-block-list]', q(`[data-preview="${zone}"]`) || document.createElement('div'));

	function pointsInput(id) {
		return q('.points-field input[name="points"]', blockForm(id) || document.createElement('div'));
	}

	// Scrolls the rail itself, never the page, so a click in the preview does not jerk the editor.
	function scrollRailTo(zone, row) {
		const list = rail(zone);
		if (!list || list.scrollHeight <= list.clientHeight) return;
		const top = row.offsetTop;
		const add = q('.rl-add', list);
		const room = list.clientHeight - (add ? add.offsetHeight : 0);
		if (top < list.scrollTop) list.scrollTop = top;
		else if (top + row.offsetHeight > list.scrollTop + room) list.scrollTop = top + row.offsetHeight - room;
	}

	function select(zone, wanted) {
		const sec = section(zone);
		if (!sec) return;
		const list = idsIn(zone);
		let key = wanted;
		if (key !== 'add' && !list.includes(key)) key = list[0] || 'empty';
		state.selected[zone] = key;

		qa('.rl-item.on, .rl-add.on', sec).forEach((el) => el.classList.remove('on'));
		qa('.block-view.sel', sec).forEach((el) => el.classList.remove('sel'));
		qa('.cf-form', sec).forEach((form) => { form.hidden = true; });

		if (key === 'add') {
			q('.rl-add', sec).classList.add('on');
			const form = q('[data-add-form]', sec);
			form.hidden = false;
			const search = q('[data-bsearch]', form);
			if (search) search.focus({ preventScroll: true });
		} else if (key === 'empty') {
			q('[data-empty-form]', sec).hidden = false;
		} else {
			const row = railItem(key);
			row.classList.add('on');
			scrollRailTo(zone, row);
			const form = blockForm(key);
			form.hidden = false;
			const position = q('[data-pos]', form);
			if (position) position.textContent = `${sec.dataset.name} · ${list.indexOf(key) + 1} of ${list.length}`;
			refreshPointsPill(key);
			const block = phoneBlock(key);
			if (block) {
				block.classList.add('sel');
				scrollPhoneTo(block);
			}
		}
	}

	function scrollPhoneTo(block) {
		const screen = block.closest('.cf-screen');
		if (!screen) return;
		const a = block.getBoundingClientRect();
		const b = screen.getBoundingClientRect();
		if (a.top >= b.top && a.bottom <= b.bottom) return;
		const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
		screen.scrollBy({ top: a.top - b.top - (b.height - a.height) / 2, behavior: reduced ? 'auto' : 'smooth' });
	}

	function totals() {
		const enabled = pointsEnabled();
		let all = 0;
		zones().forEach((zone) => {
			const items = qa('.rl-item', rail(zone) || document.createElement('div'));
			const sum = items.reduce((total, item) => total + (parseInt(item.dataset.pts, 10) || 0), 0);
			all += sum;
			const label = q('[data-total]', section(zone) || document.createElement('div'));
			if (label) {
				label.textContent = items.length
					? `${items.length} block${items.length === 1 ? '' : 's'}${enabled ? ` · ${sum} pts` : ''}`
					: 'No blocks';
			}
			const empty = q('[data-phone-empty]', section(zone) || document.createElement('div'));
			if (empty) empty.hidden = items.length > 0;
			items.forEach((item) => {
				const pts = q('.rl-pts', item);
				if (pts) pts.textContent = item.dataset.pts;
			});
		});
		const worth = q('[data-total-all]');
		if (worth) worth.textContent = `${all} point${all === 1 ? '' : 's'}`;
		hints();
	}

	// A hint for a block type is only wanted while the page has none of it.
	function hints() {
		qa('[data-hint-for]').forEach((hint) => {
			const have = qa('.rl-item', hint.closest('.obj-page')).some((item) => item.dataset.type === hint.dataset.hintFor);
			hint.hidden = have;
		});
	}

	function refreshPointsPill(id) {
		const form = blockForm(id);
		const wrap = form && q('[data-pt-wrap]', form);
		if (!wrap) return;
		const input = pointsInput(id);
		wrap.hidden = !input || !pointsEnabled();
		if (!input) return;
		const value = parseInt(input.value, 10) || 0;
		q('[data-pt-label]', wrap).textContent = `${value} pts`;
		const presets = qa('[data-set]', wrap);
		presets.forEach((chip) => chip.classList.toggle('on', parseInt(chip.dataset.set, 10) === value));
		const custom = q('[data-custom]', wrap);
		if (custom && document.activeElement !== custom) custom.value = value;
		q('.pt-input', wrap).classList.toggle('is-custom', !presets.some((chip) => parseInt(chip.dataset.set, 10) === value));
	}

	// The block's own points field is what saves, so the picker writes to it.
	function setPoints(id, value) {
		const input = pointsInput(id);
		if (!input) return;
		input.value = Math.max(0, parseInt(value, 10) || 0);
		input.dispatchEvent(new Event('input', { bubbles: true }));
		input.dispatchEvent(new Event('change', { bubbles: true }));
		const item = railItem(id);
		if (item) item.dataset.pts = input.value;
		totals();
		refreshPointsPill(id);
	}

	function railMarkers() {
		qa('.rl-item').forEach((item) => {
			const button = q('[data-var-state]', blockForm(item.dataset.id) || document.createElement('div'));
			const stateName = button ? button.dataset.varState : item.dataset.var;
			item.dataset.var = stateName || '';
			const target = q('[data-rl-var]', item);
			const icon = stateName ? q(`#rl-icon-${stateName}`) : null;
			if (target) target.innerHTML = icon ? icon.innerHTML : '';
		});
	}

	function highlight(id) {
		qa('.rl-item.hl, .block-view.hl').forEach((el) => el.classList.remove('hl'));
		if (!id) return;
		railItem(id)?.classList.add('hl');
		phoneBlock(id)?.classList.add('hl');
	}

	// The origin comes from the form: Sortable has already moved the row by drag end.
	function place(id, toZone, order) {
		const form = blockForm(id);
		const fromZone = form.dataset.zone;
		const toRail = rail(toZone);
		const add = q('.rl-add', toRail);
		const list = phoneList(toZone);
		order.forEach((blockId) => {
			const item = railItem(blockId);
			if (item) toRail.insertBefore(item, add);
			const block = phoneBlock(blockId);
			if (block && list) list.append(block);
		});
		if (fromZone !== toZone) {
			form.dataset.zone = toZone;
			q(`[data-forms="${toZone}"]`).insertBefore(form, q('[data-add-form]', q(`[data-forms="${toZone}"]`)));
		}
		return fromZone;
	}

	function save(id, fromZone, toZone, order) {
		if (fromZone === toZone) {
			window.htmx.ajax('POST', '/admin/blocks/reorder', { swap: 'none', values: { block_id: order } });
			return;
		}
		window.htmx.ajax('POST', `/admin/blocks/${id}/move`, {
			swap: 'none',
			values: { context: section(toZone).dataset.context, block_id: order },
		});
	}

	function settle(id, fromZone, toZone) {
		totals();
		railMarkers();
		if (fromZone !== toZone) {
			select(fromZone, state.selected[fromZone] === id ? null : state.selected[fromZone]);
		}
		select(toZone, id);
	}

	function orderOf(listEl) {
		return listEl.matches('[data-rail]')
			? qa('.rl-item', listEl).map((item) => item.dataset.id)
			: qa('.block-view', listEl).map((block) => block.id.replace('preview-block-', ''));
	}

	function onDrop(evt) {
		const id = evt.item.dataset.id || evt.item.id.replace('preview-block-', '');
		const toZone = evt.to.closest('[data-sec]').dataset.sec;
		const order = orderOf(evt.to);
		const fromZone = place(id, toZone, order);
		save(id, fromZone, toZone, order);
		settle(id, fromZone, toZone);
	}

	function dragOptions(draggable) {
		return {
			group: 'blocks',
			animation: 150,
			draggable,
			ghostClass: 'sortable-ghost',
			emptyInsertThreshold: 28,
			disabled: locked(),
			onEnd: onDrop,
		};
	}

	function bindRails() {
		if (!window.Sortable) return;
		zones().forEach((zone) => {
			const el = rail(zone);
			if (!el || el._sortable) return;
			el._sortable = new window.Sortable(el, dragOptions('.rl-item'));
		});
	}

	function bindPhone(pane) {
		const zone = pane.dataset.preview;
		const list = q('[data-block-list]', pane);
		if (list && window.Sortable && !list._sortable) {
			list._sortable = new window.Sortable(list, dragOptions('.block-view'));
		}
		const current = state.selected[zone];
		if (current && current !== 'add' && current !== 'empty') phoneBlock(current)?.classList.add('sel');
		totals();
	}

	function filterChooser(input) {
		const root = input.closest('.cf-pick');
		const term = input.value.trim().toLowerCase();
		let any = false;
		qa('.block-pick-item', root).forEach((item) => {
			const hit = !term || item.textContent.toLowerCase().includes(term);
			item.closest('form').hidden = !hit;
			any = any || hit;
		});
		qa('.block-pick-grid', root).forEach((grid) => {
			const has = qa('form:not([hidden])', grid).length > 0;
			grid.hidden = !has;
			if (grid.previousElementSibling) grid.previousElementSibling.hidden = !has;
		});
		q('.cf-none', root).hidden = any;
	}

	// The list item rides in a template because htmx only swaps the form.
	function adopt(forms) {
		const added = qa('template[data-rail-for]', forms);
		if (!added.length) return;
		const zone = added[0].dataset.zone;
		let chosen = null;
		added.forEach((tpl) => {
			const item = tpl.content.firstElementChild;
			rail(zone).insertBefore(item, q('.rl-add', rail(zone)));
			if (tpl.hasAttribute('data-select') || !chosen) chosen = tpl.dataset.railFor;
			tpl.remove();
		});
		const search = q('[data-bsearch]', section(zone));
		if (search) { search.value = ''; filterChooser(search); }
		totals();
		railMarkers();
		select(zone, chosen);
	}

	function askDelete(button) {
		button.closest('[data-del-wrap]').innerHTML =
			'<span class="del-ask">Delete this block? <button type="button" class="btn btn-sm" data-del-no>Keep</button><button type="button" class="btn btn-sm btn-error" data-del>Delete</button></span>';
	}

	function cancelDelete(button) {
		const wrap = button.closest('[data-del-wrap]');
		wrap.innerHTML = '<button type="button" class="btn btn-ghost btn-sm btn-square" data-del-ask aria-label="Delete block"></button>';
		q('button', wrap).append(q('#rl-icon-trash').content.cloneNode(true));
	}

	function removeBlock(id) {
		const zone = zoneOfItem(railItem(id));
		const list = idsIn(zone);
		const next = list[list.indexOf(id) + 1] || list[list.indexOf(id) - 1] || null;
		railItem(id)?.remove();
		blockForm(id)?.remove();
		phoneBlock(id)?.remove();
		totals();
		select(zone, next);
	}

	function bind() {
		if (state.bound) return;
		state.bound = true;

		document.addEventListener('click', (evt) => {
			const t = evt.target;
			if (!q('[data-objective-editor]')) return;

			const open = t.closest('.pt-wrap');
			qa('.pt-wrap.open').forEach((wrap) => { if (wrap !== open) wrap.classList.remove('open'); });

			const preset = t.closest('[data-set]');
			if (preset) {
				const form = preset.closest('[data-block-form]');
				setPoints(form.dataset.blockForm, preset.dataset.set);
				preset.closest('.pt-wrap').classList.remove('open');
				return;
			}
			const pill = t.closest('[data-pt]');
			if (pill) {
				const wrap = pill.closest('.pt-wrap');
				wrap.classList.toggle('open');
				return;
			}
			const ask = t.closest('[data-del-ask]');
			if (ask) return askDelete(ask);
			const keep = t.closest('[data-del-no]');
			if (keep) return cancelDelete(keep);
			const del = t.closest('[data-del]');
			if (del) {
				const id = del.closest('[data-block-form]').dataset.blockForm;
				window.htmx.ajax('DELETE', `/admin/blocks/${id}`, { swap: 'none', values: {} });
				return;
			}
			const add = t.closest('[data-add]');
			if (add) return select(add.closest('[data-sec]').dataset.sec, 'add');

			// A diagnostic names its block by position, so the author can go straight to it.
			const goto = t.closest('[data-lint-goto]');
			if (goto) {
				const zone = goto.closest('[data-sec]').dataset.sec;
				return select(zone, idsIn(zone)[Number(goto.dataset.lintGoto) - 1]);
			}

			const item = t.closest('.rl-item');
			if (item) return select(zoneOfItem(item), item.dataset.id);
			const block = t.closest('.block-view');
			if (block && block.closest('[data-preview]')) {
				const zone = block.closest('[data-preview]').dataset.preview;
				return select(zone, block.id.replace('preview-block-', ''));
			}
		});

		document.addEventListener('input', (evt) => {
			const t = evt.target;
			if (t.matches('[data-bsearch]')) return filterChooser(t);
			if (t.matches('[data-custom]')) return setPoints(t.closest('[data-block-form]').dataset.blockForm, t.value);
		});

		document.addEventListener('keydown', (evt) => {
			if (evt.key === 'Escape') qa('.pt-wrap.open').forEach((wrap) => wrap.classList.remove('open'));
		});

		document.addEventListener('mouseover', (evt) => {
			if (!q('[data-objective-editor]')) return;
			const block = evt.target.closest?.('.block-view');
			const item = evt.target.closest?.('.rl-item');
			highlight(block ? block.id.replace('preview-block-', '') : item ? item.dataset.id : null);
		});
		// Leaving the window is the one exit mouseover cannot see: it fires on whatever is entered.
		document.addEventListener('mouseout', (evt) => { if (!evt.relatedTarget) highlight(null); });

		document.addEventListener('change', (evt) => {
			if (evt.target.matches?.('[data-frame-toggle]')) {
				// Every preview on the page changes height, so what is above would carry the
				// switch away from the pointer. Scroll by however far it was carried.
				const switchTop = evt.target.getBoundingClientRect().top;
				applyFrame(evt.target.checked);
				const carried = evt.target.getBoundingClientRect().top - switchTop;
				if (carried) window.scrollBy(0, carried);
				try { localStorage.setItem(FRAME_KEY, evt.target.checked ? 'on' : 'off'); } catch (err) { /* not saved */ }
				return;
			}
			if (!evt.target.matches?.('.points-field input[name="points"]')) return;
			const form = evt.target.closest('[data-block-form]');
			if (!form) return;
			const item = railItem(form.dataset.blockForm);
			if (item) item.dataset.pts = evt.target.value;
			totals();
		});

		document.body.addEventListener('htmx:afterSwap', (evt) => {
			const target = evt.detail.target;
			if (!target || !q('[data-objective-editor]')) return;
			if (target.matches('[data-preview]')) bindPhone(target);
			if (target.matches('[data-forms]')) adopt(target);
			railMarkers();
		});
		document.body.addEventListener('htmx:oobAfterSwap', railMarkers);

		// htmx does not swap an error response, and the toast saying why is in it.
		document.body.addEventListener('htmx:beforeSwap', (evt) => {
			const info = evt.detail.pathInfo;
			if (info && LIST_PATH.test(info.requestPath) && evt.detail.xhr.status >= 400) {
				evt.detail.shouldSwap = true;
				evt.detail.isError = false;
			}
		});

		document.body.addEventListener('htmx:afterRequest', (evt) => {
			if (!q('[data-objective-editor]') || !evt.detail.pathInfo) return;
			const path = evt.detail.pathInfo.requestPath;
			const verb = (evt.detail.requestConfig.verb || '').toLowerCase();
			// Not `successful`: letting htmx swap an error's toast marks the request as one that worked.
			const ok = evt.detail.successful && evt.detail.xhr.status < 400;
			if (verb === 'delete' && /^\/admin\/blocks\/[^/]+$/.test(path)) {
				const id = path.split('/').pop();
				if (ok && railItem(id)) removeBlock(id);
				return;
			}
			// A refused save leaves the page showing an order the server lacks.
			if (verb === 'post' && ORDER_PATH.test(path) && !ok) {
				setTimeout(() => window.location.reload(), RELOAD_DELAY);
			}
		});

		document.body.addEventListener('quest-lock-change', (evt) => {
			qa('[data-rail]').forEach((el) => el._sortable?.option('disabled', evt.detail.locked));
			qa('[data-block-list]').forEach((el) => el._sortable?.option('disabled', evt.detail.locked));
		});
	}

	// The phone frame is a preference of whoever is editing, not of the page.
	const FRAME_KEY = 'objective-editor.frame';

	function applyFrame(on) {
		qa('.obj-page').forEach((page) => { page.dataset.frame = on ? 'on' : 'off'; });
		qa('[data-frame-toggle]').forEach((toggle) => { toggle.checked = on; });
	}

	function init() {
		bind();
		let framed = false;
		try { framed = localStorage.getItem(FRAME_KEY) === 'on'; } catch (err) { /* private windows throw */ }
		applyFrame(framed);
		state.selected = {};
		bindRails();
		totals();
		railMarkers();
		zones().forEach((zone) => select(zone, idsIn(zone)[0]));
	}

	init();
})();
