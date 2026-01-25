import React from 'react';
import { Link } from './router';
import cx from 'classnames';

export default class HighlightLink extends React.Component {
  isActive() {
    // Compare current pathname with href
    return window.location.pathname === this.props.href;
  }

  render() {
    const { activeClassName = 'active', className, ...props } = this.props;
    const finalClassName = cx(className, { [activeClassName]: this.isActive() });

    return <Link {...props} className={finalClassName} />;
  }
}

