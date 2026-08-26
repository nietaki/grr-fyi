document.addEventListener('alpine:init', () => {
  console.log('Alpine is initializing...');

  Alpine.data('combined', () => ({}));

  Alpine.data('createLink', () => ({
    targetUrl: '',
    customSlug: '',
    loading: false,
    error: null,
    result: null,

    async submit() {
      this.loading = true;
      this.error = null;
      this.result = null;

      try {
        const res = await fetch('/_/create_link', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            target_url: this.targetUrl,
            custom_slug: this.customSlug || undefined,
          }),
        });
        const data = await res.json();
        if (res.ok) {
          this.result = data;
          this.targetUrl = '';
          this.customSlug = '';
        } else {
          this.error = data.error;
        }
      } catch {
        this.error = 'Something went wrong';
      } finally {
        this.loading = false;
      }
    },
  }));
});
