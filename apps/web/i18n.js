// i18n：有翻译就替换，没有就保留页面里的中文（缺省）
const I18N = (() => {
  const STORAGE_KEY = 'xgit_web_lang';
  let lang = localStorage.getItem(STORAGE_KEY) || 'zh';

  const dict = {
    zh: {
      loginByToken:'用 Token 登录', logout:'退出登录', repos:'仓库',
      pleaseLogin:'请先登录', selectRepoTip:'选择一个仓库以浏览文件',
      welcome:'欢迎使用 XGit Web', tagline:'随时随地，轻松管理你的 GitHub 仓库。',
      backToSite:'返回官网', refresh:'刷新', viewer:'预览'
    },
    en: {
      loginByToken:'Sign in with Token', logout:'Sign out', repos:'Repositories',
      pleaseLogin:'Please sign in first', selectRepoTip:'Select a repository to browse files',
      welcome:'Welcome to XGit Web', tagline:'Manage your GitHub repos on the go.',
      backToSite:'Back to Site', refresh:'Refresh', viewer:'Viewer'
    }
  };

  function apply(container=document){
    container.querySelectorAll('[data-i18n]').forEach(el=>{
      const k = el.getAttribute('data-i18n');
      const v = dict[lang]?.[k];
      if (v) el.textContent = v; // 仅在有翻译时替换；否则保留原中文
    });
  }

  function setLang(next){
    if (!['zh','en'].includes(next)) return;
    lang = next; localStorage.setItem(STORAGE_KEY, lang); apply(document);
  }

  const urlLang = new URLSearchParams(location.search).get('lang');
  if (urlLang) setLang(urlLang);

  document.addEventListener('DOMContentLoaded', ()=>apply(document));
  return { setLang, getLang:()=>lang };
})();