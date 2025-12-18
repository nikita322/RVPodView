/**
 * VideoViewer - Viewer for video files
 * Uses HTML5 video element with controls
 */
class VideoViewer extends BaseFileViewer {
    constructor(fileData) {
        super(fileData);
        this.video = null;
    }

    async render(container) {
        this.container = container;
        container.innerHTML = '';
        container.className = 'file-viewer-container video-viewer-container';

        // Create video wrapper
        const wrapper = document.createElement('div');
        wrapper.className = 'video-viewer-wrapper';

        // Create video element
        this.video = document.createElement('video');
        this.video.className = 'file-viewer-video';
        this.video.controls = true;
        this.video.preload = 'metadata';

        // Set video source
        const source = document.createElement('source');
        source.src = this.createDataURL();
        source.type = this.fileData.mimeType || 'video/mp4';

        this.video.appendChild(source);

        // Add error handling
        this.video.onerror = () => {
            this.showError('Failed to load video. Format may not be supported.');
        };

        // Add metadata loaded handler
        this.video.onloadedmetadata = () => {
            this.updateVideoInfo();
        };

        wrapper.appendChild(this.video);
        container.appendChild(wrapper);

        // Add video info
        this.addVideoInfo(container);
    }

    addVideoInfo(container) {
        const info = document.createElement('div');
        info.className = 'video-viewer-info';
        info.id = 'video-viewer-info';
        container.appendChild(info);
    }

    updateVideoInfo() {
        const info = document.getElementById('video-viewer-info');
        if (info && this.video) {
            const duration = this.formatDuration(this.video.duration);
            const width = this.video.videoWidth;
            const height = this.video.videoHeight;
            info.textContent = `${width}×${height} • Duration: ${duration}`;
        }
    }

    formatDuration(seconds) {
        if (!seconds || isNaN(seconds)) return '0:00';

        const hours = Math.floor(seconds / 3600);
        const mins = Math.floor((seconds % 3600) / 60);
        const secs = Math.floor(seconds % 60);

        if (hours > 0) {
            return `${hours}:${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
        }
        return `${mins}:${secs.toString().padStart(2, '0')}`;
    }

    getToolbarActions() {
        const actions = super.getToolbarActions();

        actions.push({
            label: 'Fullscreen',
            icon: IconHelper.getIconHTML('maximize', '', 16),
            action: () => this.toggleFullscreen(),
            disabled: !document.fullscreenEnabled
        });

        return actions;
    }

    toggleFullscreen() {
        if (this.video) {
            if (document.fullscreenElement) {
                document.exitFullscreen();
            } else {
                this.video.requestFullscreen().catch(err => {
                    console.error('Fullscreen error:', err);
                });
            }
        }
    }

    destroy() {
        if (this.video) {
            this.video.pause();
            this.video.onerror = null;
            this.video.onloadedmetadata = null;

            // Revoke object URL if created
            const source = this.video.querySelector('source');
            if (source && source.src && source.src.startsWith('blob:')) {
                URL.revokeObjectURL(source.src);
            }

            this.video = null;
        }
        super.destroy();
    }
}

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = VideoViewer;
}
