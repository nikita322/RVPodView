/**
 * TextViewer - Viewer for plain text and code files
 * Supports editing with syntax highlighting
 */
class TextViewer extends BaseFileViewer {
    constructor(fileData) {
        super(fileData);
        this.textarea = null;
        this.originalContent = fileData.content;
        this.hasChanges = false;
        // Store bound functions for proper cleanup
        this.boundInputHandler = null;
        this.boundKeydownHandler = null;
    }

    async render(container) {
        this.container = container;
        container.innerHTML = '';
        container.className = 'file-viewer-container text-viewer-container';

        // Create textarea
        this.textarea = document.createElement('textarea');
        this.textarea.className = 'file-viewer-textarea';
        this.textarea.value = this.fileData.content;
        this.textarea.spellcheck = false;

        // Store bound handlers for proper cleanup
        this.boundInputHandler = () => {
            this.hasChanges = this.textarea.value !== this.originalContent;
            // Notify that toolbar needs update
            this.onToolbarUpdateNeeded();
        };

        this.boundKeydownHandler = (e) => {
            // Ctrl+S or Cmd+S to save
            if ((e.ctrlKey || e.metaKey) && e.key === 's') {
                e.preventDefault();
                if (this.hasChanges) {
                    this.triggerSave();
                }
            }

            // Tab key handling
            if (e.key === 'Tab') {
                e.preventDefault();
                const start = this.textarea.selectionStart;
                const end = this.textarea.selectionEnd;
                const value = this.textarea.value;

                // Insert tab
                this.textarea.value = value.substring(0, start) + '\t' + value.substring(end);
                this.textarea.selectionStart = this.textarea.selectionEnd = start + 1;
            }
        };

        // Add event listeners with bound handlers
        this.textarea.addEventListener('input', this.boundInputHandler);
        this.textarea.addEventListener('keydown', this.boundKeydownHandler);

        container.appendChild(this.textarea);

        // Focus textarea - use requestAnimationFrame for better performance
        requestAnimationFrame(() => {
            if (this.textarea) {
                this.textarea.focus();
            }
        });
    }

    // Notify parent that toolbar needs update
    onToolbarUpdateNeeded() {
        const event = new CustomEvent('file-viewer-toolbar-update');
        window.dispatchEvent(event);
    }

    canEdit() {
        return true;
    }

    getContent() {
        return this.textarea ? this.textarea.value : this.fileData.content;
    }

    setContent(content) {
        if (this.textarea) {
            this.textarea.value = content;
            this.originalContent = content;
            this.hasChanges = false;
        }
        this.fileData.content = content;
    }

    async save() {
        const content = this.getContent();

        // Will be called by the parent app
        // Just return the content for now
        return {
            path: this.fileData.path,
            content: content
        };
    }

    getToolbarActions() {
        const actions = super.getToolbarActions();

        // Add save button for editable files
        actions.unshift({
            label: 'Save',
            icon: IconHelper.getIconHTML('save', '', 16),
            action: () => this.triggerSave(),
            disabled: !this.hasChanges,
            primary: true
        });

        // Add line/char count
        actions.push({
            label: this.getStats(),
            icon: '',
            action: null,
            disabled: true,
            isInfo: true
        });

        return actions;
    }

    getStats() {
        const content = this.getContent();
        const lines = content.split('\n').length;
        const chars = content.length;
        return `${lines} lines, ${chars} chars`;
    }

    triggerSave() {
        // Dispatch custom event that parent can listen to
        const event = new CustomEvent('file-viewer-save', {
            detail: {
                viewer: this,
                path: this.fileData.path,
                content: this.getContent()
            }
        });
        window.dispatchEvent(event);
    }

    destroy() {
        if (this.textarea) {
            // Properly remove event listeners using bound functions
            if (this.boundInputHandler) {
                this.textarea.removeEventListener('input', this.boundInputHandler);
                this.boundInputHandler = null;
            }
            if (this.boundKeydownHandler) {
                this.textarea.removeEventListener('keydown', this.boundKeydownHandler);
                this.boundKeydownHandler = null;
            }
            this.textarea = null;
        }
        super.destroy();
    }
}

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = TextViewer;
}
