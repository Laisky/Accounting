// installVisualViewportHeightVar syncs CSS viewport variables with the browser's visible viewport.
export function installVisualViewportHeightVar(): () => void {
  const root = document.documentElement;
  let frame = 0;

  const update = () => {
    frame = 0;
    const viewport = window.visualViewport;
    const height = viewport?.height ?? window.innerHeight;
    const width = viewport?.width ?? window.innerWidth;
    root.style.setProperty('--app-viewport-height', `${height}px`);
    root.style.setProperty('--app-viewport-width', `${width}px`);
  };

  const requestUpdate = () => {
    if (frame) {
      return;
    }
    frame = window.requestAnimationFrame(update);
  };

  update();
  window.addEventListener('resize', requestUpdate);
  window.addEventListener('orientationchange', requestUpdate);
  window.visualViewport?.addEventListener('resize', requestUpdate);
  window.visualViewport?.addEventListener('scroll', requestUpdate);

  return () => {
    if (frame) {
      window.cancelAnimationFrame(frame);
    }
    window.removeEventListener('resize', requestUpdate);
    window.removeEventListener('orientationchange', requestUpdate);
    window.visualViewport?.removeEventListener('resize', requestUpdate);
    window.visualViewport?.removeEventListener('scroll', requestUpdate);
  };
}
