/**
 * AudioViewer - Viewer for audio files
 * Uses HTML5 audio element with custom styled controls
 */
class AudioViewer extends BaseFileViewer {
    constructor(fileData) {
        super(fileData);
        this.audio = null;
    }

    async render(container) {
        this.container = container;
        container.innerHTML = '';
        container.className = 'file-viewer-container audio-viewer-container';

        // Create audio player wrapper
        const wrapper = document.createElement('div');
        wrapper.className = 'audio-viewer-wrapper';

        // Add file icon
        const icon = document.createElement('div');
        icon.className = 'audio-viewer-icon';
        icon.appendChild(IconHelper.createIcon('music', '', 64));
        wrapper.appendChild(icon);

        // Add file name
        const fileName = document.createElement('div');
        fileName.className = 'audio-viewer-filename';
        fileName.textContent = this.fileData.name;
        wrapper.appendChild(fileName);

        // Create audio element
        this.audio = document.createElement('audio');
        this.audio.className = 'file-viewer-audio';
        this.audio.controls = true;
        this.audio.preload = 'metadata';

        // Set audio source
        const source = document.createElement('source');
        source.src = this.createDataURL();
        source.type = this.fileData.mimeType || 'audio/mpeg';

        this.audio.appendChild(source);

        // Add error handling
        this.audio.onerror = () => {
            this.showError('Failed to load audio. Format may not be supported.');
        };

        // Add metadata loaded handler
        this.audio.onloadedmetadata = () => {
            this.updateAudioInfo();
        };

        wrapper.appendChild(this.audio);
        container.appendChild(wrapper);

        // Add audio info
        this.addAudioInfo(container);
    }

    addAudioInfo(container) {
        const info = document.createElement('div');
        info.className = 'audio-viewer-info';
        info.id = 'audio-viewer-info';
        container.appendChild(info);
    }

    updateAudioInfo() {
        const info = document.getElementById('audio-viewer-info');
        if (info && this.audio) {
            const duration = this.formatDuration(this.audio.duration);
            const size = this.formatFileSize(this.fileData.size);
            info.textContent = `Duration: ${duration} • Size: ${size}`;
        }
    }

    formatDuration(seconds) {
        if (!seconds || isNaN(seconds)) return '0:00';

        const mins = Math.floor(seconds / 60);
        const secs = Math.floor(seconds % 60);
        return `${mins}:${secs.toString().padStart(2, '0')}`;
    }

    getToolbarActions() {
        const actions = super.getToolbarActions();

        const isPaused = !this.audio || this.audio.paused;
        actions.push({
            label: isPaused ? 'Play' : 'Pause',
            icon: IconHelper.getIconHTML(isPaused ? 'play' : 'pause', '', 16),
            action: () => this.togglePlay(),
            disabled: false
        });

        return actions;
    }

    togglePlay() {
        if (this.audio) {
            if (this.audio.paused) {
                this.audio.play().catch(err => {
                    console.error('Play error:', err);
                });
            } else {
                this.audio.pause();
            }
        }
    }

    destroy() {
        if (this.audio) {
            this.audio.pause();
            this.audio.onerror = null;
            this.audio.onloadedmetadata = null;

            // Revoke object URL if created
            const source = this.audio.querySelector('source');
            if (source && source.src && source.src.startsWith('blob:')) {
                URL.revokeObjectURL(source.src);
            }

            this.audio = null;
        }
        super.destroy();
    }
}

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = AudioViewer;
}
