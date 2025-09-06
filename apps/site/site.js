// Minimal i18n toggler (CN/EN) + persistence
const $  = (s)=>document.querySelector(s);
const $$ = (s)=>document.querySelectorAll(s);

function setLang(lang){
  document.documentElement.setAttribute('lang', lang);
  localStorage.setItem('xgit_lang', lang);
  // toggle every .i18n child spans
  $$('.i18n').forEach(el=>{
    el.querySelectorAll('[data-zh],[data-en]').forEach(node=>{
      node.style.display = (node.dataset[lang] !== undefined) ? '' : 'none';
    });
  });
  // active buttons
  $('#btnZh')?.classList.toggle('active', lang==='zh');
  $('#btnEn')?.classList.toggle('active', lang==='en');
}

function initLang(){
  const saved = localStorage.getItem('xgit_lang');
  const lang = saved || ((navigator.language||'zh').toLowerCase().startsWith('zh') ? 'zh' : 'en');
  setLang(lang);
}

function bind(){
  $('#btnZh')?.addEventListener('click', ()=>setLang('zh'));
  $('#btnEn')?.addEventListener('click', ()=>setLang('en'));
}

document.addEventListener('DOMContentLoaded', ()=>{
  bind();
  initLang();
});