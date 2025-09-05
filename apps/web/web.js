(function(){
  const KEY='xgit_web_lang';
  let lang = localStorage.getItem(KEY) || ((navigator.language||'').toLowerCase().startsWith('zh')?'zh':'en');
  const dict = { zh:{back:'回到官网',repos:'仓库',tree:'文件',editor:'编辑器',changes:'更改',commit:'提交'},
                 en:{back:'Back to Site',repos:'Repos',tree:'Tree',editor:'Editor',changes:'Changes',commit:'Commit'} };
  function t(k){ return (dict[lang]&&dict[lang][k])||k; }
  function apply(){ document.querySelectorAll('[data-i18n]').forEach(el=> el.textContent=t(el.dataset.i18n)); }
  function setLang(l){ lang=l; localStorage.setItem(KEY,l); apply(); }

  const repos = [ { name:'ximory/xgit-demo', branches:['main'], files:[
      { path:'README.md', text:'# Demo\n\nHello XGit!' },
      { path:'apps/web/web.js', text:'// todo: implement GitHub API' },
      { path:'docs/changelog.md', text:'# changelog\n- v0.1.0 init' }
    ] } ];
  let current = {repo:null, file:null};
  const els = { repos:document.getElementById('repos'), tree:document.getElementById('tree'),
    editor:document.getElementById('editor'), changes:document.getElementById('changes'),
    msg:document.getElementById('msg'), commit:document.getElementById('commitBtn'),
    zh:document.getElementById('langZh'), en:document.getElementById('langEn') };
  const changes = new Map(); // path -> {old,new,staged}

  function renderRepos(){
    els.repos.innerHTML = repos.map((r,i)=>`<li data-i="${i}">${r.name}</li>`).join('');
    els.repos.querySelectorAll('li').forEach(li=> li.onclick=()=>{ current.repo = repos[li.dataset.i|0]; renderTree(); });
  }
  function renderTree(){
    const list = current.repo?.files||[];
    els.tree.innerHTML = list.map((f,i)=>`<li data-i="${i}">${f.path}</li>`).join('');
    els.tree.querySelectorAll('li').forEach(li=> li.onclick=()=>{
      const f = current.repo.files[li.dataset.i|0]; current.file=f;
      els.editor.value = f.text;
      changes.set(f.path, {old:f.text, new:f.text, staged:false});
      refreshChanges();
    });
  }
  function refreshChanges(){
    const rows=[];
    for(const [path,c] of changes){
      if(c.new!==c.old){
        rows.push(`<div class="change-row">
          <label><input type="checkbox" data-path="${path}" ${c.staged?'checked':''}> ${path}</label>
          <div><button data-edit="${path}">Edit</button></div>
        </div>`);
      }
    }
    els.changes.innerHTML = rows.length? rows.join('') : `<div style="color:#888;font-size:13px">No changes</div>`;
    els.changes.querySelectorAll('input[type=checkbox]').forEach(chk=> chk.onchange=()=>{ const c=changes.get(chk.dataset.path); if(c){c.staged=chk.checked;} });
    els.changes.querySelectorAll('button[data-edit]').forEach(b=> b.onclick=()=>{ const p=b.dataset.edit; const c=changes.get(p); if(c){ els.editor.value=c.new; } });
  }
  els.editor.addEventListener('input', ()=>{
    if(!current.file) return;
    const c = changes.get(current.file.path) || {old:current.file.text, new:current.file.text, staged:false};
    c.new = els.editor.value;
    if(c.new===c.old){ changes.delete(current.file.path); } else { changes.set(current.file.path, c); }
    refreshChanges();
  });
  els.commit.onclick = ()=>{
    const staged=[...changes.entries()].filter(([_,c])=>c.staged);
    if(!staged.length){ alert(lang==='zh'?'没有选中要提交的文件':'No staged files'); return; }
    const msg = els.msg.value.trim() || (lang==='zh'?'更新 via XGit':'update via XGit');
    console.log('COMMIT', {message:msg, files: staged.map(([p,c])=>({path:p, len:c.new.length}))});
    alert((lang==='zh'?'提交成功，文件数：':'Committed files: ')+staged.length);
    staged.forEach(([p,c])=>{ c.old=c.new; c.staged=false; });
    refreshChanges();
  };
  els.zh.onclick=()=>setLang('zh'); els.en.onclick=()=>setLang('en');
  document.addEventListener('DOMContentLoaded', ()=>{ apply(); renderRepos(); });
})();