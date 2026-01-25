import React from 'react';
import Router from 'react-router-component';
import cx from 'classnames';

class HighlightLink extends React.Component {
    static contextType = Router.NavigatableMixin;

    isActive() {
        // getPath() returns the path of the active Location in the current router.
        return this.getPath && this.getPath() === this.props.href
    }

    render() {
        const { activeClassName = 'active', className } = this.props;
        const finalClassName = cx(className, { [activeClassName]: this.isActive() });

        return (
            <Router.Link {...this.props} className={finalClassName} />
        );
    }
}

export default HighlightLink;
