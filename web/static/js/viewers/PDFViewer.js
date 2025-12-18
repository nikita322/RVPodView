/**
 * PDFViewer - Viewer for PDF files
 * Uses iframe or object element to embed PDF
 */
class PDFViewer extends BaseFileViewer {
    constructor(fileData) {
        super(fileData);
        this.embedElement = null;
    }

    async render(container) {
        this.container = container;
        container.innerHTML = '';
        container.className = 'file-viewer-container pdf-viewer-container';

        // Try to use object element first (better compatibility)
        this.embedElement = document.createElement('object');
        this.embedElement.className = 'file-viewer-pdf';
        this.embedElement.type = 'application/pdf';
        this.embedElement.data = this.createDataURL();

        // Fallback message if PDF plugin not available
        const fallback = document.createElement('div');
        fallback.className = 'pdf-viewer-fallback';

        const fallbackIcon = document.createElement('div');
        fallbackIcon.className = 'fallback-icon';
        fallbackIcon.appendChild(IconHelper.createIcon('file', '', 48));

        const fallbackMessage = document.createElement('div');
        fallbackMessage.className = 'fallback-message';
        fallbackMessage.innerHTML = `
            <p>PDF viewer not available in your browser.</p>
            <button class="btn btn-primary" onclick="this.getRootNode().host.download()">
                Download PDF
            </button>
        `;

        fallback.appendChild(fallbackIcon);
        fallback.appendChild(fallbackMessage);

        this.embedElement.appendChild(fallback);
        container.appendChild(this.embedElement);

        // Add info
        this.addPDFInfo(container);
    }

    addPDFInfo(container) {
        const info = document.createElement('div');
        info.className = 'pdf-viewer-info';
        info.textContent = `PDF Document • ${this.formatFileSize(this.fileData.size)}`;
        container.appendChild(info);
    }

    getToolbarActions() {
        const actions = super.getToolbarActions();

        actions.push({
            label: 'Open in New Tab',
            icon: IconHelper.getIconHTML('external-link', '', 16),
            action: () => this.openInNewTab(),
            disabled: false
        });

        return actions;
    }

    openInNewTab() {
        const url = this.createDataURL();
        window.open(url, '_blank');
    }

    destroy() {
        if (this.embedElement) {
            // Revoke object URL if created
            if (this.embedElement.data && this.embedElement.data.startsWith('blob:')) {
                URL.revokeObjectURL(this.embedElement.data);
            }
            this.embedElement = null;
        }
        super.destroy();
    }
}

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = PDFViewer;
}
