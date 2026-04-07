const TITLE_RULES = [
  { min: 4000, value: () => null },
  { min: 3000, value: 'Legendary Grandmaster' },
  { min: 2600, value: 'International Grandmaster' },
  { min: 2400, value: 'Grandmaster' },
  { min: 2300, value: 'International Master' },
  { min: 2100, value: 'Master' },
  { min: 1900, value: 'Candidate Master' },
  { min: 1600, value: 'Expert' },
  { min: 1400, value: 'Specialist' },
  { min: 1200, value: 'Pupil' },
  { min: 0, value: 'Newbie' },
];

const COLOR_RULES = [
  { min: 2400, value: '#FF0000' },
  { min: 2100, value: '#FF8C00' },
  { min: 1900, value: '#AA00AA' },
  { min: 1600, value: '#0000FF' },
  { min: 1400, value: '#03A89E' },
  { min: 1200, value: '#008000' },
  { min: 0, value: '#808080' },
];

const state = {
  payload: null,
  selectedIndex: 0,
};

const elements = {
  title: document.querySelector('#app-title'),
  metaUpdated: document.querySelector('#meta-updated'),
  tabList: document.querySelector('#tab-list'),
  stateBanner: document.querySelector('#state-banner'),
  userView: document.querySelector('#user-view'),
  profileAvatar: document.querySelector('#profile-avatar'),
  profileName: document.querySelector('#profile-name'),
  profileRating: document.querySelector('#profile-rating'),
  profileSummary: document.querySelector('#profile-summary'),
  contestList: document.querySelector('#contest-list'),
  contestTemplate: document.querySelector('#contest-template'),
  problemTemplate: document.querySelector('#problem-template'),
};

async function boot() {
  try {
    const response = await fetch('./data/dashboard.json', { cache: 'no-store' });
    const payload = await response.json();

    if (!response.ok) {
      showBanner(payload.message || 'Dashboard is warming up.');
      return;
    }

    state.payload = payload;
    renderApp();
  } catch (error) {
    console.error(error);
    showBanner('Failed to load dashboard data.');
  }
}

function renderApp() {
  if (!state.payload) {
    return;
  }

  document.title = state.payload.title;
  elements.title.textContent = state.payload.title;
  elements.metaUpdated.textContent = state.payload.updatedAtLabel;

  hideBanner();
  renderTabs();
  renderUser();
  elements.userView.classList.remove('hidden');
}

function renderTabs() {
  elements.tabList.innerHTML = '';
  const fragment = document.createDocumentFragment();

  state.payload.users.forEach((user, index) => {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'tab-button';
    button.role = 'tab';
    button.textContent = user.handle;
    button.setAttribute('aria-selected', String(index === state.selectedIndex));
    button.addEventListener('click', () => {
      if (index === state.selectedIndex) {
        return;
      }
      state.selectedIndex = index;
      renderTabs();
      renderUser();
    });
    fragment.appendChild(button);
  });

  elements.tabList.appendChild(fragment);
}

function renderUser() {
  const user = state.payload.users[state.selectedIndex];
  if (!user) {
    return;
  }

  const color = ratingColor(user.currentRating.value);
  const maxColor = ratingColor(user.maxRating.value);
  const currentTitle = ratingTitle(user.currentRating.value) || user.currentRating.rank;
  const maxTitle = ratingTitle(user.maxRating.value) || user.maxRating.rank;

  if (user.avatarUrl) {
    elements.profileAvatar.src = user.avatarUrl;
    elements.profileAvatar.alt = `${user.handle} avatar`;
    elements.profileAvatar.classList.remove('hidden');
  } else {
    elements.profileAvatar.removeAttribute('src');
    elements.profileAvatar.alt = '';
    elements.profileAvatar.classList.add('hidden');
  }

  elements.profileName.textContent = user.handle;
  elements.profileName.style.color = color;
  elements.profileRating.innerHTML = [
    `<span class="rating-current" style="color:${escapeAttribute(color)}">${escapeHtml(currentTitle)}, ${escapeHtml(String(user.currentRating.value))}</span>`,
    `<span class="rating-neutral"> </span>`,
    `<span class="rating-neutral">(max. </span>`,
    `<span class="rating-max" style="color:${escapeAttribute(maxColor)}">${escapeHtml(maxTitle)}, ${escapeHtml(String(user.maxRating.value))}</span>`,
    `<span class="rating-neutral">)</span>`,
  ].join('');

  elements.profileSummary.innerHTML = '';
  elements.profileSummary.appendChild(summaryTile('Contests', String(user.contests.length)));

  renderContests(user.contests);
}

function renderContests(contests) {
  elements.contestList.innerHTML = '';

  if (!contests.length) {
    const empty = document.createElement('div');
    empty.className = 'empty-state';
    empty.textContent = 'No eligible virtual participations were found for this handle.';
    elements.contestList.appendChild(empty);
    return;
  }

  const fragment = document.createDocumentFragment();
  contests.forEach((contest, rowIndex) => {
    const node = elements.contestTemplate.content.firstElementChild.cloneNode(true);
    node.style.animationDelay = `${rowIndex * 35}ms`;

    const contestLink = node.querySelector('.contest-link');
    contestLink.href = contest.url;
    contestLink.textContent = contest.name;

    const contestMeta = node.querySelector('.contest-meta');
    const metaUrl = contest.friendsStandingsUrl || contest.url;
    const lines = [];
    if (contest.startedAt) {
      lines.push(`<a href="${escapeAttribute(metaUrl)}" target="_blank" rel="noreferrer noopener">Contest: ${escapeHtml(contest.startedAt)}</a>`);
    }
    lines.push(`<a href="${escapeAttribute(metaUrl)}" target="_blank" rel="noreferrer noopener">Virtual: ${escapeHtml(contest.virtualAt)}</a>`);
    contestMeta.innerHTML = lines.map((line) => `<div>${line}</div>`).join('');

    const strip = node.querySelector('.problem-strip');
    contest.problems.forEach((problem) => {
      const cell = elements.problemTemplate.content.firstElementChild.cloneNode(true);
      const problemIndex = cell.querySelector('.problem-index');
      const problemLink = cell.querySelector('.problem-pill-link');
      const problemPill = cell.querySelector('.problem-pill');

      problemIndex.href = problem.url;
      problemIndex.textContent = problem.index;
      problemIndex.title = problem.name || problem.index;

      problemLink.href = problem.url;
      problemLink.title = `${problem.index}: ${problem.name || 'Unknown problem'} (${labelForStatus(problem.status)})`;

      problemPill.classList.add(problem.status);
      problemPill.textContent = problem.status === 'pending' ? '' : problem.attemptsLabel;

      strip.appendChild(cell);
    });

    fragment.appendChild(node);
  });

  elements.contestList.appendChild(fragment);
}

function summaryTile(label, value) {
  const tile = document.createElement('div');
  tile.className = 'summary-tile';
  const caption = document.createElement('span');
  caption.textContent = label;
  const strong = document.createElement('strong');
  strong.textContent = value;
  tile.append(caption, strong);
  return tile;
}

function showBanner(message) {
  elements.stateBanner.textContent = message;
  elements.stateBanner.classList.remove('hidden');
}

function hideBanner() {
  elements.stateBanner.classList.add('hidden');
}

function ratingTitle(rating) {
  for (const rule of TITLE_RULES) {
    if (rating >= rule.min) {
      return typeof rule.value === 'function' ? rule.value(rating) : rule.value;
    }
  }
  return 'Newbie';
}

function ratingColor(rating) {
  for (const rule of COLOR_RULES) {
    if (rating >= rule.min) {
      return rule.value;
    }
  }
  return '#808080';
}

function labelForStatus(status) {
  switch (status) {
    case 'virtual':
      return 'accepted during virtual participation';
    case 'upsolved':
      return 'accepted after the virtual participation';
    case 'attempt':
      return 'attempted but not accepted';
    default:
      return 'not attempted';
  }
}

function escapeHtml(value) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function escapeAttribute(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;');
}

boot();
