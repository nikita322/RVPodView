/**
 * ImageViewer - Viewer for image files
 * Supports zoom, pan, and rotation
 */
class ImageViewer extends BaseFileViewer {
    constructor(fileData) {
        super(fileData);
        this.img = null;
        this.zoom = 1;
        this.rotation = 0;
    }

    async render(container) {
        this.container = container;
        container.innerHTML = '';
        container.className = 'file-viewer-container image-viewer-container';

        // Create image wrapper
        const wrapper = document.createElement('div');
        wrapper.className = 'image-viewer-wrapper';

        // Create image element
        this.img = document.createElement('img');
        this.img.className = 'file-viewer-image';
        this.img.alt = this.fileData.name;

        // Set image source
        this.img.src = this.createDataURL();

        // Add load error handling
        this.img.onerror = () => {
            this.showError('Failed to load image');
        };

        // Add load handler to show dimensions
        this.img.onload = () => {
            this.updateImageInfo();
        };

        wrapper.appendChild(this.img);
        container.appendChild(wrapper);

        // Add image controls
        this.addImageControls(container);
    }

    addImageControls(container) {
        const controls = document.createElement('div');
        controls.className = 'image-viewer-controls';

        // Zoom controls
        const zoomIn = this.createIconButton('zoom-in', 'Zoom In', () => this.setZoom(this.zoom + 0.25));
        const zoomOut = this.createIconButton('zoom-out', 'Zoom Out', () => this.setZoom(this.zoom - 0.25));
        const zoomReset = this.createIconButton('refresh', 'Reset Zoom', () => this.resetView());

        // Rotation controls
        const rotateLeft = this.createIconButton('rotate-ccw', 'Rotate Left', () => this.rotate(-90));
        const rotateRight = this.createIconButton('rotate-cw', 'Rotate Right', () => this.rotate(90));

        // Info display
        const info = document.createElement('span');
        info.className = 'image-viewer-info';
        info.id = 'image-viewer-info';

        controls.appendChild(zoomOut);
        controls.appendChild(zoomIn);
        controls.appendChild(zoomReset);
        controls.appendChild(rotateLeft);
        controls.appendChild(rotateRight);
        controls.appendChild(info);

        container.appendChild(controls);
    }

    createIconButton(iconName, title, onClick) {
        const btn = document.createElement('button');
        btn.className = 'image-viewer-btn';
        btn.appendChild(IconHelper.createIcon(iconName, '', 18));
        btn.title = title;
        btn.onclick = onClick;
        return btn;
    }

    setZoom(newZoom) {
        this.zoom = Math.max(0.25, Math.min(5, newZoom));
        this.updateTransform();
    }

    rotate(degrees) {
        this.rotation = (this.rotation + degrees) % 360;
        this.updateTransform();
    }

    resetView() {
        this.zoom = 1;
        this.rotation = 0;
        this.updateTransform();
    }

    updateTransform() {
        if (this.img) {
            this.img.style.transform = `scale(${this.zoom}) rotate(${this.rotation}deg)`;
            this.updateImageInfo();
        }
    }

    updateImageInfo() {
        const info = document.getElementById('image-viewer-info');
        if (info && this.img) {
            const width = this.img.naturalWidth;
            const height = this.img.naturalHeight;
            const zoomPercent = Math.round(this.zoom * 100);
            info.textContent = `${width}×${height} • ${zoomPercent}% • ${this.rotation}°`;
        }
    }

    getToolbarActions() {
        // Only return download action from parent
        // All image controls (zoom, rotate) are in the overlay at the bottom
        return super.getToolbarActions();
    }

    destroy() {
        if (this.img) {
            this.img.onerror = null;
            this.img.onload = null;
            // Revoke object URL if created
            if (this.img.src && this.img.src.startsWith('blob:')) {
                URL.revokeObjectURL(this.img.src);
            }
            this.img = null;
        }
        super.destroy();
    }
}

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = ImageViewer;
}
