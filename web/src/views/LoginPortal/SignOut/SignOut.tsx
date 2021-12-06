import React, { useEffect, useCallback, useState } from "react";

import { Typography, makeStyles } from "@material-ui/core";
import { Navigate } from "react-router-dom";

import { FirstFactorRoute } from "@constants/Routes";
import { useIsMountedRef } from "@hooks/Mounted";
import { useNotifications } from "@hooks/NotificationsContext";
import { useRedirectionURL } from "@hooks/RedirectionURL";
import { useRedirector } from "@hooks/Redirector";
import LoginLayout from "@layouts/LoginLayout";
import { signOut } from "@services/SignOut";

export interface Props {}

const SignOut = function (props: Props) {
    const mounted = useIsMountedRef();
    const style = useStyles();
    const { createErrorNotification } = useNotifications();
    const redirectionURL = useRedirectionURL();
    const redirector = useRedirector();
    const [timedOut, setTimedOut] = useState(false);
    const [safeRedirect, setSafeRedirect] = useState(false);

    const doSignOut = useCallback(async () => {
        try {
            const res = await signOut(redirectionURL);
            if (res !== undefined && res.safeTargetURL) {
                setSafeRedirect(true);
            }
            setTimeout(() => {
                if (!mounted) {
                    return;
                }
                setTimedOut(true);
            }, 2000);
        } catch (err) {
            console.error(err);
            createErrorNotification("There was an issue signing out");
        }
    }, [createErrorNotification, redirectionURL, setSafeRedirect, setTimedOut, mounted]);

    useEffect(() => {
        doSignOut();
    }, [doSignOut]);

    if (timedOut) {
        if (redirectionURL && safeRedirect) {
            redirector(redirectionURL);
        } else {
            return <Navigate to={FirstFactorRoute} />;
        }
    }

    return (
        <LoginLayout title="Sign out">
            <Typography className={style.typo}>You're being signed out and redirected...</Typography>
        </LoginLayout>
    );
};

export default SignOut;

const useStyles = makeStyles((theme) => ({
    typo: {
        padding: theme.spacing(),
    },
}));
