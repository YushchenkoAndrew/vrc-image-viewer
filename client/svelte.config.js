import adapter from '@sveltejs/adapter-static';

export default {
  kit: {
    adapter: adapter({
      pages: 'build',
      assets: 'build',
      fallback: 'index.html', // SPA fallback — needed since /images and /copy are external
      precompress: false,
      strict: false
    })
  }
};
