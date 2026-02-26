document.addEventListener('alpine:init', () => {
  console.log('Alpine is initializing...');

  Alpine.data('combined', () => ({}));
});
