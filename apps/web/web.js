/* simplified: only show updated rendering block for repo cards */
async function loadRepos(){
  const token = localStorage.getItem('xgit_token');
  if(!token){ return; }

  let repos = [];
  try{
    repos = await apiRepos({ per_page: 100, page: 1 });
  }catch(e){
    console.warn(e);
    $('#repoList').classList.remove('hidden');
    $('#repoList').innerHTML = `<li>${esc(lang==='zh'?'加载仓库失败':'Failed to load repositories')}</li>`;
    return;
  }

  $('#repoList').classList.remove('hidden');
  if(!repos || repos.length===0){
    $('#repoList').innerHTML = `<li>${esc(t('noRepos'))}</li>`;
    return;
  }

  const html = repos.map(r=>{
    const privacy = r.private ? '🔒' : '🌐';
    const full = `${r.owner?.login || ''}/${r.name}`;
    const langTag = r.language ? `<span class="tag">${esc(r.language)}</span>` : '';
    const br = r.default_branch || 'main';
    const updated = r.pushed_at ? new Date(r.pushed_at).toLocaleString() : '';
    const desc = r.description ? `<div class="desc">${esc(r.description)}</div>` : '';
    return `<li class="repo" data-owner="${esc(r.owner.login)}" data-repo="${esc(r.name)}" data-branch="${esc(br)}">
      <div class="row">
        <div class="left">
          <div class="name">${privacy} ${esc(full)}</div>
          ${desc}
          <div class="meta">
            ${langTag}
            <span class="tag">branch: ${esc(br)}</span>
            <span class="tag">updated: ${esc(updated)}</span>
          </div>
        </div>
        <div class="right">
          <a class="chip" href="${esc(r.html_url)}" target="_blank" rel="noreferrer">GitHub</a>
        </div>
      </div>
    </li>`;
  }).join('');

  $('#repoList').innerHTML = html;
}
